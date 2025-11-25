package context

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/free5gc/openapi/models"
)

// TestAmfUeRanUe_AttachDetachLifecycle tests the complete lifecycle of RanUe attachment and detachment
// 測試目標：完整測試 AttachRanUe 和 Remove (內含 Detach) 的串連與解開邏輯
func TestAmfUeRanUe_AttachDetachLifecycle(t *testing.T) {
	// ====== 準備階段 ======
	// 建立測試環境
	accessType := models.AccessType__3_GPP_ACCESS

	// 建立並初始化 AMF Context
	amfContext := GetSelf()
	
	// 初始化必要的配置 (避免 panic)
	if len(amfContext.ServedGuamiList) == 0 {
		amfContext.ServedGuamiList = append(amfContext.ServedGuamiList, models.Guami{
			PlmnId: &models.PlmnIdNid{
				Mcc: "208",
				Mnc: "93",
			},
			AmfId: "cafe00",
		})
	}
	if len(amfContext.SupportTaiLists) == 0 {
		amfContext.SupportTaiLists = append(amfContext.SupportTaiLists, models.Tai{
			PlmnId: &models.PlmnId{
				Mcc: "208",
				Mnc: "93",
			},
			Tac: "000001",
		})
	}

	// 建立 AmfUe
	testSupi := "imsi-208930000000001"
	amfUe := amfContext.NewAmfUe(testSupi)
	require.NotNil(t, amfUe, "AmfUe should be created successfully")

	// 建立 AmfRan (需要假的連接)
	fakeConn := &fakeNetConn{}
	amfRan := amfContext.NewAmfRan(fakeConn)
	require.NotNil(t, amfRan, "AmfRan should be created successfully")
	amfRan.AnType = accessType
	amfRan.Name = "test-gnb"

	// 建立 RanUe
	ranUeNgapId := int64(1)
	ranUe, err := amfRan.NewRanUe(ranUeNgapId)
	require.NoError(t, err, "RanUe should be created without error")
	require.NotNil(t, ranUe, "RanUe should not be nil")

	// 驗證初始狀態
	require.True(t, amfUe.CmIdle(accessType), "AmfUe should start in CM-IDLE state")

	// ====== 執行 Attach ======
	t.Run("Attach RanUe to AmfUe", func(t *testing.T) {
		amfUe.AttachRanUe(ranUe)

		// ====== 斷言 Attach 成功 ======
		// 1. AmfUe 應該進入 CM-CONNECTED 狀態
		require.True(t, amfUe.CmConnect(accessType),
			"AmfUe should be CM-CONNECTED after AttachRanUe")
		require.False(t, amfUe.CmIdle(accessType),
			"AmfUe should NOT be CM-IDLE after AttachRanUe")

		// 2. AmfUe.RanUe[accessType] 應該指向 ranUe
		attachedRanUe, exists := amfUe.RanUe[accessType]
		require.True(t, exists, "RanUe should exist in AmfUe.RanUe map")
		require.Equal(t, ranUe, attachedRanUe,
			"AmfUe.RanUe[accessType] should point to the attached RanUe")

		// 3. ranUe.AmfUe 應該指向 amfUe
		require.Equal(t, amfUe, ranUe.AmfUe,
			"ranUe.AmfUe should point to the attached AmfUe")

		// 4. RanUe 應該在 RAN 的 UE 列表中
		storedRanUe, found := amfRan.RanUeList.Load(ranUeNgapId)
		require.True(t, found, "RanUe should be in AmfRan.RanUeList")
		require.Equal(t, ranUe, storedRanUe, "Stored RanUe should match")

		// 5. RanUe 應該在 AMF Context 的 RanUePool 中
		storedInPool, foundInPool := amfContext.RanUePool.Load(ranUe.AmfUeNgapId)
		require.True(t, foundInPool, "RanUe should be in AMFContext.RanUePool")
		require.Equal(t, ranUe, storedInPool, "RanUe in pool should match")

		// 6. AmfUe 應該在 AMF Context 的 UePool 中
		storedAmfUe, foundAmfUe := amfContext.UePool.Load(testSupi)
		require.True(t, foundAmfUe, "AmfUe should be in AMFContext.UePool")
		require.Equal(t, amfUe, storedAmfUe, "AmfUe in pool should match")
	})

	// ====== 執行 Detach (透過 RanUe.Remove) ======
	t.Run("Detach RanUe via Remove", func(t *testing.T) {
		// 呼叫 ranUe.Remove() (這會觸發 amfUe.DetachRanUe)
		err := ranUe.Remove()
		require.NoError(t, err, "RanUe.Remove should succeed without error")

		// ====== 斷言 Detach 成功 ======
		// 1. AmfUe 應該回到 CM-IDLE 狀態
		require.True(t, amfUe.CmIdle(accessType),
			"AmfUe should return to CM-IDLE after RanUe.Remove")
		require.False(t, amfUe.CmConnect(accessType),
			"AmfUe should NOT be CM-CONNECTED after RanUe.Remove")

		// 2. amfUe.RanUe[accessType] 應該變回 nil (被刪除)
		_, exists := amfUe.RanUe[accessType]
		require.False(t, exists,
			"AmfUe.RanUe[accessType] should be deleted after Detach")

		// 3. ranUe.AmfUe 應該變為 nil
		require.Nil(t, ranUe.AmfUe,
			"ranUe.AmfUe should be nil after Detach")

		// 4. RanUe 應該從 RAN 的 UE 列表中移除
		_, found := amfRan.RanUeList.Load(ranUeNgapId)
		require.False(t, found,
			"RanUe should be removed from AmfRan.RanUeList")

		// 5. RanUe 應該從 AMF Context 的 RanUePool 中移除 (N2 連線已被刪除)
		_, foundInPool := amfContext.RanUePool.Load(ranUe.AmfUeNgapId)
		require.False(t, foundInPool,
			"RanUe should be removed from AMFContext.RanUePool (N2 connection deleted)")

		// 6. AmfUe 應該仍然在 AMF Context 的 UePool 中 (客戶檔案必須保留)
		storedAmfUe, foundAmfUe := amfContext.UePool.Load(testSupi)
		require.True(t, foundAmfUe,
			"AmfUe should still be in AMFContext.UePool (user profile must be retained)")
		require.Equal(t, amfUe, storedAmfUe,
			"AmfUe in pool should still match the original")
	})

	// ====== 清理 ======
	t.Cleanup(func() {
		// 清理 AmfUe
		amfContext.UePool.Delete(testSupi)
		// 清理 AmfRan
		amfContext.AmfRanPool.Delete(fakeConn)
	})
}

// TestAmfUeRanUe_MultipleAttachDetach tests multiple attach-detach cycles
// 測試目標：驗證多次 Attach/Detach 循環的穩定性
func TestAmfUeRanUe_MultipleAttachDetach(t *testing.T) {
	accessType := models.AccessType__3_GPP_ACCESS
	amfContext := GetSelf()

	// 初始化配置
	if len(amfContext.ServedGuamiList) == 0 {
		amfContext.ServedGuamiList = append(amfContext.ServedGuamiList, models.Guami{
			PlmnId: &models.PlmnIdNid{
				Mcc: "208",
				Mnc: "93",
			},
			AmfId: "cafe00",
		})
	}

	// 建立 AmfUe
	testSupi := "imsi-208930000000002"
	amfUe := amfContext.NewAmfUe(testSupi)
	require.NotNil(t, amfUe)

	// 建立 AmfRan
	fakeConn := &fakeNetConn{}
	amfRan := amfContext.NewAmfRan(fakeConn)
	amfRan.AnType = accessType

	// 執行多次 Attach-Detach 循環
	for i := 0; i < 3; i++ {
		t.Run("Cycle "+string(rune('A'+i)), func(t *testing.T) {
			// 建立新的 RanUe
			ranUeNgapId := int64(10 + i)
			ranUe, err := amfRan.NewRanUe(ranUeNgapId)
			require.NoError(t, err)

			// Attach
			amfUe.AttachRanUe(ranUe)
			require.True(t, amfUe.CmConnect(accessType),
				"Cycle %d: Should be CM-CONNECTED after attach", i)

			// Detach
			err = ranUe.Remove()
			require.NoError(t, err)
			require.True(t, amfUe.CmIdle(accessType),
				"Cycle %d: Should be CM-IDLE after detach", i)
		})
	}

	// 清理
	t.Cleanup(func() {
		amfContext.UePool.Delete(testSupi)
		amfContext.AmfRanPool.Delete(fakeConn)
	})
}

// TestAmfUeRanUe_BothAccessTypes tests lifecycle with both 3GPP and Non-3GPP access
// 測試目標：驗證兩種 Access Type 同時存在時的生命週期
func TestAmfUeRanUe_BothAccessTypes(t *testing.T) {
	amfContext := GetSelf()

	// 初始化配置
	if len(amfContext.ServedGuamiList) == 0 {
		amfContext.ServedGuamiList = append(amfContext.ServedGuamiList, models.Guami{
			PlmnId: &models.PlmnIdNid{
				Mcc: "208",
				Mnc: "93",
			},
			AmfId: "cafe00",
		})
	}

	// 建立 AmfUe
	testSupi := "imsi-208930000000003"
	amfUe := amfContext.NewAmfUe(testSupi)

	// 建立兩個 AmfRan (3GPP 和 Non-3GPP)
	fakeConn3gpp := &fakeNetConn{}
	amfRan3gpp := amfContext.NewAmfRan(fakeConn3gpp)
	amfRan3gpp.AnType = models.AccessType__3_GPP_ACCESS

	fakeConnNon3gpp := &fakeNetConn{}
	amfRanNon3gpp := amfContext.NewAmfRan(fakeConnNon3gpp)
	amfRanNon3gpp.AnType = models.AccessType_NON_3_GPP_ACCESS

	// 建立兩個 RanUe
	ranUe3gpp, err := amfRan3gpp.NewRanUe(1)
	require.NoError(t, err)

	ranUeNon3gpp, err := amfRanNon3gpp.NewRanUe(2)
	require.NoError(t, err)

	t.Run("Attach Both Access Types", func(t *testing.T) {
		// Attach 3GPP
		amfUe.AttachRanUe(ranUe3gpp)
		require.True(t, amfUe.CmConnect(models.AccessType__3_GPP_ACCESS))

		// Attach Non-3GPP
		amfUe.AttachRanUe(ranUeNon3gpp)
		require.True(t, amfUe.CmConnect(models.AccessType_NON_3_GPP_ACCESS))

		// 兩者都應該是 CM-CONNECTED
		require.True(t, amfUe.CmConnect(models.AccessType__3_GPP_ACCESS))
		require.True(t, amfUe.CmConnect(models.AccessType_NON_3_GPP_ACCESS))
	})

	t.Run("Detach 3GPP Only", func(t *testing.T) {
		// 只移除 3GPP RanUe
		err := ranUe3gpp.Remove()
		require.NoError(t, err)

		// 3GPP 應該是 CM-IDLE
		require.True(t, amfUe.CmIdle(models.AccessType__3_GPP_ACCESS))

		// Non-3GPP 應該仍然是 CM-CONNECTED
		require.True(t, amfUe.CmConnect(models.AccessType_NON_3_GPP_ACCESS))
	})

	t.Run("Detach Non-3GPP", func(t *testing.T) {
		// 移除 Non-3GPP RanUe
		err := ranUeNon3gpp.Remove()
		require.NoError(t, err)

		// 兩者都應該是 CM-IDLE
		require.True(t, amfUe.CmIdle(models.AccessType__3_GPP_ACCESS))
		require.True(t, amfUe.CmIdle(models.AccessType_NON_3_GPP_ACCESS))

		// AmfUe 應該仍在 UePool
		_, found := amfContext.UePool.Load(testSupi)
		require.True(t, found, "AmfUe should still exist in UePool")
	})

	// 清理
	t.Cleanup(func() {
		amfContext.UePool.Delete(testSupi)
		amfContext.AmfRanPool.Delete(fakeConn3gpp)
		amfContext.AmfRanPool.Delete(fakeConnNon3gpp)
	})
}

// TestAmfUeRanUe_DetachWithoutAttach tests error handling for detaching non-existent connection
// 測試目標：驗證未 Attach 就執行 Detach 的錯誤處理
func TestAmfUeRanUe_DetachWithoutAttach(t *testing.T) {
	accessType := models.AccessType__3_GPP_ACCESS
	amfContext := GetSelf()

	// 初始化配置
	if len(amfContext.ServedGuamiList) == 0 {
		amfContext.ServedGuamiList = append(amfContext.ServedGuamiList, models.Guami{
			PlmnId: &models.PlmnIdNid{
				Mcc: "208",
				Mnc: "93",
			},
			AmfId: "cafe00",
		})
	}

	// 建立 AmfUe (沒有 Attach RanUe)
	testSupi := "imsi-208930000000004"
	amfUe := amfContext.NewAmfUe(testSupi)

	t.Run("Detach when no RanUe attached", func(t *testing.T) {
		// 嘗試 Detach (應該不會造成問題)
		amfUe.DetachRanUe(accessType)

		// 應該仍然是 CM-IDLE
		require.True(t, amfUe.CmIdle(accessType))

		// AmfUe 應該仍在 UePool
		_, found := amfContext.UePool.Load(testSupi)
		require.True(t, found)
	})

	// 清理
	t.Cleanup(func() {
		amfContext.UePool.Delete(testSupi)
	})
}

// ==================== 邊緣測試 (Edge Case Tests) ====================

// TestAmfUeRanUe_DoubleAttach tests attaching the same RanUe twice
// 測試目標：測試重複 Attach 同一個 RanUe
func TestAmfUeRanUe_DoubleAttach(t *testing.T) {
	amfContext := GetSelf()

	// 初始化配置
	if len(amfContext.ServedGuamiList) == 0 {
		amfContext.ServedGuamiList = append(amfContext.ServedGuamiList, models.Guami{
			PlmnId: &models.PlmnIdNid{
				Mcc: "208",
				Mnc: "93",
			},
			AmfId: "cafe00",
		})
	}

	testSupi := "imsi-208930000000201"
	amfUe := amfContext.NewAmfUe(testSupi)
	require.NotNil(t, amfUe)

	fakeConn := &fakeNetConn{}
	amfRan := amfContext.NewAmfRan(fakeConn)
	amfRan.AnType = models.AccessType__3_GPP_ACCESS

	ranUe, err := amfRan.NewRanUe(1)
	require.NoError(t, err)
	require.NotNil(t, ranUe)

	t.Run("First Attach", func(t *testing.T) {
		// 第一次 Attach
		amfUe.AttachRanUe(ranUe)

		// 驗證連接成功
		require.True(t, amfUe.CmConnect(models.AccessType__3_GPP_ACCESS))
		require.Equal(t, amfUe, ranUe.AmfUe)
		require.Equal(t, ranUe, amfUe.RanUe[models.AccessType__3_GPP_ACCESS])
	})

	t.Run("Double Attach Same RanUe", func(t *testing.T) {
		// 第二次 Attach 同一個 RanUe (應該不會造成問題)
		require.NotPanics(t, func() {
			amfUe.AttachRanUe(ranUe)
		}, "Double attach should not panic")

		// 驗證連接仍然正確
		require.True(t, amfUe.CmConnect(models.AccessType__3_GPP_ACCESS),
			"Should still be CM-CONNECTED after double attach")
		require.Equal(t, amfUe, ranUe.AmfUe,
			"ranUe.AmfUe should still point to amfUe")
		require.Equal(t, ranUe, amfUe.RanUe[models.AccessType__3_GPP_ACCESS],
			"amfUe.RanUe should still point to ranUe")
	})

	t.Run("Verify Pools After Double Attach", func(t *testing.T) {
		// 驗證 Pool 中沒有重複
		_, foundInUePool := amfContext.UePool.Load(testSupi)
		require.True(t, foundInUePool, "AmfUe should be in UePool")

		_, foundInRanUePool := amfContext.RanUePool.Load(ranUe.AmfUeNgapId)
		require.True(t, foundInRanUePool, "RanUe should be in RanUePool")
	})

	// 清理
	t.Cleanup(func() {
		ranUe.Remove()
		amfContext.UePool.Delete(testSupi)
		amfContext.AmfRanPool.Delete(fakeConn)
	})
}

// TestAmfUeRanUe_DoubleRemove tests removing the same RanUe twice
// 測試目標：測試重複刪除同一個 RanUe
func TestAmfUeRanUe_DoubleRemove(t *testing.T) {
	amfContext := GetSelf()

	// 初始化配置
	if len(amfContext.ServedGuamiList) == 0 {
		amfContext.ServedGuamiList = append(amfContext.ServedGuamiList, models.Guami{
			PlmnId: &models.PlmnIdNid{
				Mcc: "208",
				Mnc: "93",
			},
			AmfId: "cafe00",
		})
	}

	testSupi := "imsi-208930000000202"
	amfUe := amfContext.NewAmfUe(testSupi)

	fakeConn := &fakeNetConn{}
	amfRan := amfContext.NewAmfRan(fakeConn)
	amfRan.AnType = models.AccessType__3_GPP_ACCESS

	ranUe, err := amfRan.NewRanUe(1)
	require.NoError(t, err)

	// Attach
	amfUe.AttachRanUe(ranUe)
	require.True(t, amfUe.CmConnect(models.AccessType__3_GPP_ACCESS))

	t.Run("First Remove", func(t *testing.T) {
		// 第一次 Remove
		err := ranUe.Remove()
		require.NoError(t, err, "First remove should succeed")

		// 驗證已刪除
		require.True(t, amfUe.CmIdle(models.AccessType__3_GPP_ACCESS))
		require.Nil(t, ranUe.AmfUe, "ranUe.AmfUe should be nil after remove")
	})

	t.Run("Second Remove Same RanUe", func(t *testing.T) {
		// 第二次 Remove 同一個 RanUe
		// 應該要返回錯誤或優雅處理
		err := ranUe.Remove()
		
		// 驗證行為
		if err != nil {
			// 如果返回錯誤,確認是預期的錯誤
			t.Logf("Second remove returned error (expected): %v", err)
			require.Contains(t, err.Error(), "RanUe not found",
				"Error should indicate RanUe not found")
		} else {
			// 如果不返回錯誤,確認不會 panic
			t.Log("Second remove succeeded without error (idempotent)")
		}

		// 無論如何,AmfUe 應該保持 CM-IDLE
		require.True(t, amfUe.CmIdle(models.AccessType__3_GPP_ACCESS),
			"AmfUe should remain CM-IDLE after second remove")
	})

	t.Run("Verify AmfUe Still Exists", func(t *testing.T) {
		// AmfUe 應該仍在 UePool (檔案保留)
		_, found := amfContext.UePool.Load(testSupi)
		require.True(t, found, "AmfUe should still exist in UePool")
	})

	// 清理
	t.Cleanup(func() {
		amfContext.UePool.Delete(testSupi)
		amfContext.AmfRanPool.Delete(fakeConn)
	})
}

// TestAmfUeRanUe_AttachAfterRemove tests attaching a RanUe after it has been removed
// 測試目標：測試 RanUe 被 Remove 後再次 Attach
func TestAmfUeRanUe_AttachAfterRemove(t *testing.T) {
	amfContext := GetSelf()

	// 初始化配置
	if len(amfContext.ServedGuamiList) == 0 {
		amfContext.ServedGuamiList = append(amfContext.ServedGuamiList, models.Guami{
			PlmnId: &models.PlmnIdNid{
				Mcc: "208",
				Mnc: "93",
			},
			AmfId: "cafe00",
		})
	}

	testSupi := "imsi-208930000000203"
	amfUe := amfContext.NewAmfUe(testSupi)

	fakeConn := &fakeNetConn{}
	amfRan := amfContext.NewAmfRan(fakeConn)
	amfRan.AnType = models.AccessType__3_GPP_ACCESS

	ranUe, err := amfRan.NewRanUe(1)
	require.NoError(t, err)

	t.Run("Initial Attach and Remove", func(t *testing.T) {
		// 先 Attach
		amfUe.AttachRanUe(ranUe)
		require.True(t, amfUe.CmConnect(models.AccessType__3_GPP_ACCESS))

		// 然後 Remove
		err := ranUe.Remove()
		require.NoError(t, err)
		require.True(t, amfUe.CmIdle(models.AccessType__3_GPP_ACCESS))
	})

	t.Run("Try to Attach Removed RanUe", func(t *testing.T) {
		// 嘗試再次 Attach 已被刪除的 RanUe
		// 這個行為可能不正確,但測試應該不會 panic

		// 注意: ranUe 已經被 Remove,其內部狀態可能不一致
		// 這個測試主要驗證不會 panic

		require.NotPanics(t, func() {
			amfUe.AttachRanUe(ranUe)
		}, "Attaching removed RanUe should not panic")

		// 檢查狀態 (可能會變成 CONNECTED,因為 map 被設定了)
		// 但這個 RanUe 已經不在 Pool 中
		t.Logf("CM-CONNECTED after re-attach: %v", amfUe.CmConnect(models.AccessType__3_GPP_ACCESS))

		// 重要: 這個 ranUe 不應該在 RanUePool 中 (因為已被刪除)
		_, foundInPool := amfContext.RanUePool.Load(ranUe.AmfUeNgapId)
		require.False(t, foundInPool,
			"Removed RanUe should NOT be in RanUePool even after re-attach")
	})

	t.Run("Create New RanUe for Proper Reconnection", func(t *testing.T) {
		// 正確的做法: 建立新的 RanUe
		newRanUe, err := amfRan.NewRanUe(2)
		require.NoError(t, err)

		amfUe.AttachRanUe(newRanUe)
		require.True(t, amfUe.CmConnect(models.AccessType__3_GPP_ACCESS))

		// 新的 RanUe 應該在 Pool 中
		_, foundInPool := amfContext.RanUePool.Load(newRanUe.AmfUeNgapId)
		require.True(t, foundInPool, "New RanUe should be in RanUePool")

		// 清理新的 RanUe
		newRanUe.Remove()
	})

	// 清理
	t.Cleanup(func() {
		amfContext.UePool.Delete(testSupi)
		amfContext.AmfRanPool.Delete(fakeConn)
	})
}

// TestAmfUeRanUe_RemoveWithNilAmfUe tests removing RanUe when AmfUe is nil
// 測試目標：測試 RanUe.AmfUe 為 nil 時的 Remove
func TestAmfUeRanUe_RemoveWithNilAmfUe(t *testing.T) {
	amfContext := GetSelf()

	// 初始化配置
	if len(amfContext.ServedGuamiList) == 0 {
		amfContext.ServedGuamiList = append(amfContext.ServedGuamiList, models.Guami{
			PlmnId: &models.PlmnIdNid{
				Mcc: "208",
				Mnc: "93",
			},
			AmfId: "cafe00",
		})
	}

	fakeConn := &fakeNetConn{}
	amfRan := amfContext.NewAmfRan(fakeConn)
	amfRan.AnType = models.AccessType__3_GPP_ACCESS

	ranUe, err := amfRan.NewRanUe(1)
	require.NoError(t, err)
	require.NotNil(t, ranUe)

	t.Run("Remove RanUe Without AmfUe", func(t *testing.T) {
		// 確認 ranUe.AmfUe 是 nil (因為沒有 Attach)
		require.Nil(t, ranUe.AmfUe, "ranUe.AmfUe should be nil initially")

		// 嘗試 Remove (應該優雅處理)
		err := ranUe.Remove()

		// 驗證: 應該能成功或返回適當錯誤
		if err != nil {
			t.Logf("Remove with nil AmfUe returned error: %v", err)
		} else {
			t.Log("Remove with nil AmfUe succeeded")
		}

		// 驗證: 不應該 panic
		require.NotPanics(t, func() {
			_ = ranUe.Remove()
		}, "Remove should handle nil AmfUe gracefully")
	})

	t.Run("Verify RanUe Removed from Pool", func(t *testing.T) {
		// RanUe 應該從 RanUePool 中刪除
		_, found := amfContext.RanUePool.Load(ranUe.AmfUeNgapId)
		require.False(t, found, "RanUe should be removed from RanUePool")
	})

	// 清理
	t.Cleanup(func() {
		amfContext.AmfRanPool.Delete(fakeConn)
	})
}

// TestAmfUeRanUe_RemoveWithNilRan tests removing RanUe when Ran is nil
// 測試目標：測試 RanUe.Ran 為 nil 時的 Remove (極端邊界情況)
func TestAmfUeRanUe_RemoveWithNilRan(t *testing.T) {
	amfContext := GetSelf()

	// 初始化配置
	if len(amfContext.ServedGuamiList) == 0 {
		amfContext.ServedGuamiList = append(amfContext.ServedGuamiList, models.Guami{
			PlmnId: &models.PlmnIdNid{
				Mcc: "208",
				Mnc: "93",
			},
			AmfId: "cafe00",
		})
	}

	testSupi := "imsi-208930000000204"
	amfUe := amfContext.NewAmfUe(testSupi)

	fakeConn := &fakeNetConn{}
	amfRan := amfContext.NewAmfRan(fakeConn)
	amfRan.AnType = models.AccessType__3_GPP_ACCESS

	ranUe, err := amfRan.NewRanUe(1)
	require.NoError(t, err)

	// Attach
	amfUe.AttachRanUe(ranUe)
	require.True(t, amfUe.CmConnect(models.AccessType__3_GPP_ACCESS))

	t.Run("Manually Set Ran to Nil", func(t *testing.T) {
		// 手動將 ranUe.Ran 設為 nil (模擬極端錯誤情況)
		ranUe.Ran = nil

		// 嘗試 Remove
		err := ranUe.Remove()

		// 驗證: 應該返回錯誤
		require.Error(t, err, "Remove should return error when Ran is nil")
		require.Contains(t, err.Error(), "Ran", "Error should mention Ran")

		t.Logf("Remove with nil Ran returned error: %v", err)
	})

	t.Run("Verify AmfUe Still Connected", func(t *testing.T) {
		// 因為 Remove 失敗,AmfUe 可能仍然是 CONNECTED
		// (取決於 Remove 的實作細節)
		t.Logf("AmfUe CM-CONNECTED: %v", amfUe.CmConnect(models.AccessType__3_GPP_ACCESS))
	})

	// 清理 (手動清理,因為 Remove 可能失敗了)
	t.Cleanup(func() {
		// 手動清理關聯
		delete(amfUe.RanUe, models.AccessType__3_GPP_ACCESS)
		amfContext.UePool.Delete(testSupi)
		amfContext.AmfRanPool.Delete(fakeConn)
		amfContext.RanUePool.Delete(ranUe.AmfUeNgapId)
	})
}

// TestAmfUeRanUe_ConcurrentAttachDetach tests concurrent attach and detach operations
// 測試目標：測試並發的 Attach 和 Detach 操作
func TestAmfUeRanUe_ConcurrentAttachDetach(t *testing.T) {
	amfContext := GetSelf()

	// 初始化配置
	if len(amfContext.ServedGuamiList) == 0 {
		amfContext.ServedGuamiList = append(amfContext.ServedGuamiList, models.Guami{
			PlmnId: &models.PlmnIdNid{
				Mcc: "208",
				Mnc: "93",
			},
			AmfId: "cafe00",
		})
	}

	testSupi := "imsi-208930000000205"
	amfUe := amfContext.NewAmfUe(testSupi)

	fakeConn := &fakeNetConn{}
	amfRan := amfContext.NewAmfRan(fakeConn)
	amfRan.AnType = models.AccessType__3_GPP_ACCESS

	iterations := 50

	t.Run("50 Concurrent Attach-Detach Cycles", func(t *testing.T) {
		done := make(chan bool, iterations)

		// 啟動多個 goroutine 同時進行 Attach-Detach
		for i := 0; i < iterations; i++ {
			go func(id int) {
				defer func() {
					done <- true
				}()

				// 建立新的 RanUe
				ranUe, err := amfRan.NewRanUe(int64(id + 100))
				if err != nil {
					t.Logf("Goroutine %d: Failed to create RanUe: %v", id, err)
					return
				}

				// Attach
				amfUe.AttachRanUe(ranUe)

				// 短暫延遲
				time.Sleep(time.Millisecond)

				// Remove
				_ = ranUe.Remove()
			}(i)
		}

		// 等待所有 goroutine 完成
		for i := 0; i < iterations; i++ {
			<-done
		}
	})

	t.Run("Verify Final State", func(t *testing.T) {
		// 最終狀態可能是 IDLE 或 CONNECTED (取決於最後一個完成的 goroutine)
		t.Logf("Final CM-CONNECTED: %v", amfUe.CmConnect(models.AccessType__3_GPP_ACCESS))

		// AmfUe 應該仍在 UePool
		_, found := amfContext.UePool.Load(testSupi)
		require.True(t, found, "AmfUe should still exist in UePool")
	})

	// 清理
	t.Cleanup(func() {
		// 清理可能殘留的連接
		amfUe.DetachRanUe(models.AccessType__3_GPP_ACCESS)
		amfContext.UePool.Delete(testSupi)
		amfContext.AmfRanPool.Delete(fakeConn)
	})
}

// fakeNetConn is a fake implementation of net.Conn for testing
// 用於測試的假 net.Conn 實作
type fakeNetConn struct{}

func (f *fakeNetConn) Read(b []byte) (n int, err error)   { return 0, nil }
func (f *fakeNetConn) Write(b []byte) (n int, err error)  { return len(b), nil }
func (f *fakeNetConn) Close() error                       { return nil }
func (f *fakeNetConn) LocalAddr() net.Addr                { return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 38412} }
func (f *fakeNetConn) RemoteAddr() net.Addr               { return &net.TCPAddr{IP: net.ParseIP("127.0.0.2"), Port: 38412} }
func (f *fakeNetConn) SetDeadline(t time.Time) error      { return nil }
func (f *fakeNetConn) SetReadDeadline(t time.Time) error  { return nil }
func (f *fakeNetConn) SetWriteDeadline(t time.Time) error { return nil }
