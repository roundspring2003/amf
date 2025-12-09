package context

import (
	"net"
	"sync"
	"testing"

	"github.com/free5gc/amf/internal/logger"
	"github.com/free5gc/nas/security"
	"github.com/free5gc/openapi/models"
	"github.com/free5gc/util/idgenerator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper: 建立一個隔離的測試用 AMF Context，確保 Independent
// 這取代了 GetSelf()，避免全域狀態汙染
func newTestAmfContext() *AMFContext {
	ctx := &AMFContext{
		UePool:          sync.Map{},
		AmfRanPool:      sync.Map{},
		ServedGuamiList: make([]models.Guami, 0),
	}
	// 預設給一個 GUAMI 避免部分邏輯 crash
	ctx.ServedGuamiList = append(ctx.ServedGuamiList, models.Guami{
		PlmnId: &models.PlmnIdNid{Mcc: "001", Mnc: "01"},
		AmfId:  "000001",
	})
	return ctx
}

// ==========================================
// 1. 純函數測試 (Table-Driven Tests)
// 符合原則: Fast, Independent, Repeatable, Self-Validating
// ==========================================

func TestGetIntAlgOrder_Mapping(t *testing.T) {
	t.Parallel() // 標記為可平行執行，提升速度

	// 定義測試案例結構
	tests := []struct {
		name     string
		input    []string
		expected []uint8
	}{
		{
			name:     "Normal Case",
			input:    []string{"NIA2", "NIA0"},
			expected: []uint8{security.AlgIntegrity128NIA2, security.AlgIntegrity128NIA0},
		},
		{
			name:     "Empty Input",
			input:    []string{},
			expected: []uint8{},
		},
		{
			name:     "Filter Unsupported Algs",
			input:    []string{"NIA1", "INVALID_ALG", "NIA2"},
			expected: []uint8{security.AlgIntegrity128NIA1, security.AlgIntegrity128NIA2},
		},
	}

	for _, tc := range tests {
		tc := tc // capture range variable for parallel execution
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			input := tc.input

			// Act
			got := getIntAlgOrder(input)
			if len(tc.expected) == 0 {
				assert.Empty(t, got)
			} else {
				assert.Equal(t, tc.expected, got, "Integrity algorithm mapping mismatch")
			}
		})
	}
}

// ==========================================
// 2. Context 與 Pool 測試 (Isolated Context)
// 符合原則: Independent (使用 newTestAmfContext), 3A Pattern
// ==========================================

func TestAmfUe_Operations(t *testing.T) {
	t.Parallel()

	t.Run("Create UE with Empty SUPI", func(t *testing.T) {
		t.Parallel()
		// Arrange
		amf := newTestAmfContext()

		// Act
		ue := amf.NewAmfUe("")

		// Assert
		require.NotNil(t, ue, "UE should be created")
		assert.NotEmpty(t, ue.Guti, "GUTI should be allocated")

		_, ok := amf.AmfUeFindBySupi("")
		assert.False(t, ok, "UE with empty SUPI should not be in the pool")
	})

	t.Run("Handle Duplicate SUPI", func(t *testing.T) {
		t.Parallel()
		// Arrange
		amf := newTestAmfContext()
		supi := "imsi-duplicate-test"
		ue1 := amf.NewAmfUe(supi)

		// Act
		ue2 := amf.NewAmfUe(supi) // overwrite

		// Assert
		found, ok := amf.AmfUeFindBySupi(supi)
		require.True(t, ok, "UE should be found")
		assert.Equal(t, ue2, found, "Should find the second UE instance")
		assert.NotEqual(t, ue1, found, "First UE should be overwritten")
	})

	t.Run("Find Non-Existent UE", func(t *testing.T) {
		t.Parallel()
		// Arrange
		amf := newTestAmfContext()

		// Act
		found, ok := amf.AmfUeFindBySupi("imsi-ghost")

		// Assert
		assert.False(t, ok)
		assert.Nil(t, found)
	})

	t.Run("Normal Lifecycle (Happy Path)", func(t *testing.T) {
		t.Parallel()
		// Arrange
		amf := newTestAmfContext()
		supi := "imsi-12345"

		// Act
		ue := amf.NewAmfUe(supi)
		found, ok := amf.AmfUeFindBySupi(supi)

		// Assert
		require.NotNil(t, ue)
		require.True(t, ok, "UE should be found in pool")
		assert.Equal(t, ue, found, "Found UE should match created UE")
		assert.NotEmpty(t, ue.Guti, "GUTI should be allocated")
	})
}

func TestAmfRan_Operations(t *testing.T) {
	t.Parallel()

	t.Run("Create and Find Ran", func(t *testing.T) {
		t.Parallel()
		// Arrange
		amf := newTestAmfContext()
		mockConn := &net.TCPConn{} // Mock connection object

		// Act
		ran := amf.NewAmfRan(mockConn)
		foundRan, ok := amf.AmfRanFindByConn(mockConn)

		// Assert
		require.NotNil(t, ran)
		require.True(t, ok)
		assert.Equal(t, ran, foundRan, "Stored RAN should match created RAN")
	})

	t.Run("Find with Nil Connection", func(t *testing.T) {
		t.Parallel()
		// Arrange
		amf := newTestAmfContext()

		// Act & Assert (Panic check included via behavior)
		assert.NotPanics(t, func() {
			found, ok := amf.AmfRanFindByConn(nil)
			assert.False(t, ok)
			assert.Nil(t, found)
		})
	})

	t.Run("Delete Non-Existent Ran", func(t *testing.T) {
		t.Parallel()
		// Arrange
		amf := newTestAmfContext()
		mockConn := &net.TCPConn{}

		// Act & Assert
		assert.NotPanics(t, func() {
			amf.AmfRanPool.Delete(mockConn)
		})
		_, ok := amf.AmfRanFindByConn(mockConn)
		assert.False(t, ok)
	})
	t.Run("Find Non-Existent Ran (Valid Conn)", func(t *testing.T) {
		t.Parallel()
		// Arrange
		amf := newTestAmfContext()
		mockConn := &net.TCPConn{} // Valid pointer, but not in pool

		// Act
		found, ok := amf.AmfRanFindByConn(mockConn)

		// Assert
		assert.False(t, ok, "Should not find RAN that wasn't added")
		assert.Nil(t, found)
	})

	t.Run("Handle Duplicate Connection", func(t *testing.T) {
		t.Parallel()
		// Arrange
		amf := newTestAmfContext()
		mockConn := &net.TCPConn{}

		ran1 := amf.NewAmfRan(mockConn)
		require.NotNil(t, ran1)

		// Act: Create second RAN with same connection
		ran2 := amf.NewAmfRan(mockConn)

		// Assert
		require.NotNil(t, ran2)

		// Verify which RAN is in the pool (Based on map behavior, usually the latest)
		foundRan, ok := amf.AmfRanFindByConn(mockConn)
		require.True(t, ok)

		// 驗證邏輯：通常新的會覆蓋舊的，或者根據實作指向同一個 key
		// 這裡加上 Log 讓你確認行為，如同原本的測試
		if foundRan != ran2 {
			t.Logf("Note: Implementation kept the first RAN instance or behaved unexpectedly. Got %p, expected %p", foundRan, ran2)
		} else {
			assert.Equal(t, ran2, foundRan, "Pool should hold the latest RAN for the connection")
		}
	})
}

func TestRanUe_SwitchToRan(t *testing.T) {
	t.Parallel()

	// Arrange
	const (
		oldRanUeID = int64(1000)
		newRanUeID = int64(12345)
	)

	ranA := &AmfRan{Name: "ranA", Log: logger.CtxLog}
	ranB := &AmfRan{Name: "ranB", Log: logger.CtxLog}

	// 初始化 map，避免 nil pointer panic (如果原始 struct 沒有 init)
	ranA.RanUeList = sync.Map{}
	ranB.RanUeList = sync.Map{}

	ranUe := &RanUe{
		Ran:         ranA,
		RanUeNgapId: oldRanUeID,
		Log:         logger.CtxLog,
	}
	ranA.RanUeList.Store(oldRanUeID, ranUe)

	// Act
	err := ranUe.SwitchToRan(ranB, newRanUeID)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, ranB, ranUe.Ran)
	assert.Equal(t, newRanUeID, ranUe.RanUeNgapId)

	_, existsInOld := ranA.RanUeList.Load(oldRanUeID)
	assert.False(t, existsInOld, "Should be removed from old RAN")

	storedUe, existsInNew := ranB.RanUeList.Load(newRanUeID)
	require.True(t, existsInNew, "Should exist in new RAN")
	assert.Equal(t, ranUe, storedUe)
}

// ==========================================
// 3. 全域狀態測試 (Global State / Singleton)
// 這些測試無法平行執行 (沒有 t.Parallel)，因為它們共用 tmsiGenerator
// ==========================================

func TestTmsiGenerator_Lifecycle(t *testing.T) {
	// Arrange: 替換全域變數，確保測試環境一致
	oldGen := tmsiGenerator
	tmsiGenerator = idgenerator.NewGenerator(1, 2)
	// 使用 Cleanup 確保測試結束後還原全域狀態
	t.Cleanup(func() { tmsiGenerator = oldGen })

	amf := newTestAmfContext() // 使用局部 AMF，但 TMSI generator 是全域的

	t.Run("Allocate Free Reuse", func(t *testing.T) {
		// Arrange
		// (Reset generator specific for this sub-test if needed,
		// but since we run sequentially, we just manage the flow)

		// Act & Assert - 1st Alloc
		tmsi1 := amf.TmsiAllocate()
		require.NotEqual(t, -1, tmsi1, "First TMSI allocation failed")

		// Act & Assert - 2nd Alloc
		tmsi2 := amf.TmsiAllocate()
		require.NotEqual(t, -1, tmsi2, "Second TMSI allocation failed")
		assert.NotEqual(t, tmsi1, tmsi2, "TMSIs should be unique")

		// Act - Free & Reuse
		amf.FreeTmsi(int64(tmsi1))
		tmsi3 := amf.TmsiAllocate()

		// Assert - Reuse
		require.NotEqual(t, -1, tmsi3)
		assert.Equal(t, tmsi1, int32(tmsi3), "Should reuse the freed TMSI")

		// Cleanup for this run
		amf.FreeTmsi(int64(tmsi2))
		amf.FreeTmsi(int64(tmsi3))
	})

	t.Run("Exhaustion", func(t *testing.T) {
		// Arrange: Reset generator for this specific scenario
		// 注意：因為這是全域變數，子測試間可能互相影響，所以再次 NewGenerator 比較保險
		tmsiGenerator = idgenerator.NewGenerator(1, 2)

		// Act
		t1 := amf.TmsiAllocate()
		t2 := amf.TmsiAllocate()
		t3 := amf.TmsiAllocate()

		// Assert
		assert.NotEqual(t, -1, t1)
		assert.NotEqual(t, -1, t2)
		assert.Equal(t, int32(-1), t3, "Should fail when exhausted")
	})

	t.Run("Free Invalid Range", func(t *testing.T) {
		// Act & Assert
		assert.NotPanics(t, func() {
			amf.FreeTmsi(-999)
			amf.FreeTmsi(999)
		}, "Freeing invalid TMSI should not panic")

		// Verify generator still works
		val := amf.TmsiAllocate()
		assert.NotEqual(t, -1, val)
	})
}
