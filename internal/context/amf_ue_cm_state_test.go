package context

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/free5gc/amf/internal/logger"
	"github.com/free5gc/openapi/models"
)

// TestAmfUeCmState_InitialState tests that a newly initialized AmfUe is in CM-IDLE state
// 測試目標：驗證新建立的 AmfUe 初始狀態為 CM-IDLE
func TestAmfUeCmState_InitialState(t *testing.T) {
	// 準備：建立一個 AmfUe 物件並初始化
	ue := &AmfUe{}
	ue.init()

	// 測試 3GPP Access
	t.Run("3GPP Access - Initial CM-IDLE State", func(t *testing.T) {
		// 斷言：初始狀態下應該是 CM-IDLE
		require.True(t, ue.CmIdle(models.AccessType__3_GPP_ACCESS),
			"Newly initialized AmfUe should be in CM-IDLE state for 3GPP access")

		// 斷言：初始狀態下不應該是 CM-CONNECTED
		require.False(t, ue.CmConnect(models.AccessType__3_GPP_ACCESS),
			"Newly initialized AmfUe should NOT be in CM-CONNECTED state for 3GPP access")
	})

	// 測試 Non-3GPP Access
	t.Run("Non-3GPP Access - Initial CM-IDLE State", func(t *testing.T) {
		// 斷言：初始狀態下應該是 CM-IDLE
		require.True(t, ue.CmIdle(models.AccessType_NON_3_GPP_ACCESS),
			"Newly initialized AmfUe should be in CM-IDLE state for Non-3GPP access")

		// 斷言：初始狀態下不應該是 CM-CONNECTED
		require.False(t, ue.CmConnect(models.AccessType_NON_3_GPP_ACCESS),
			"Newly initialized AmfUe should NOT be in CM-CONNECTED state for Non-3GPP access")
	})
}

// TestAmfUeCmState_AfterRanUeAttach tests CM state after attaching a RanUe
// 測試目標：驗證連接 RanUe 後，AmfUe 狀態變為 CM-CONNECTED
func TestAmfUeCmState_AfterRanUeAttach(t *testing.T) {
	// 準備：建立 AmfUe 並初始化
	ue := &AmfUe{}
	ue.init()

	// 驗證初始狀態
	require.True(t, ue.CmIdle(models.AccessType__3_GPP_ACCESS),
		"AmfUe should start in CM-IDLE state")

	t.Run("Attach RanUe - Transition to CM-CONNECTED", func(t *testing.T) {
		// 手動設定 RanUe (模擬連線建立)
		// 建立一個假的 RanUe 物件
		fakeRanUe := &RanUe{
			RanUeNgapId: 1,
			AmfUeNgapId: 1,
			Log:         logger.NgapLog.WithField("test", "cm-state"),
		}

		// 將 RanUe 連接到 AmfUe
		ue.RanUe[models.AccessType__3_GPP_ACCESS] = fakeRanUe

		// 斷言：現在應該是 CM-CONNECTED 狀態
		require.True(t, ue.CmConnect(models.AccessType__3_GPP_ACCESS),
			"AmfUe should be in CM-CONNECTED state after RanUe is attached")

		// 斷言：不應該是 CM-IDLE 狀態
		require.False(t, ue.CmIdle(models.AccessType__3_GPP_ACCESS),
			"AmfUe should NOT be in CM-IDLE state when RanUe is attached")
	})
}

// TestAmfUeCmState_MultiAccessType tests CM state for different access types independently
// 測試目標：驗證不同 Access Type 的 CM 狀態是獨立的
func TestAmfUeCmState_MultiAccessType(t *testing.T) {
	// 準備：建立 AmfUe 並初始化
	ue := &AmfUe{}
	ue.init()

	t.Run("Independent CM State per Access Type", func(t *testing.T) {
		// 只連接 3GPP Access 的 RanUe
		fakeRanUe3gpp := &RanUe{
			RanUeNgapId: 1,
			AmfUeNgapId: 1,
			Log:         logger.NgapLog.WithField("test", "3gpp"),
		}
		ue.RanUe[models.AccessType__3_GPP_ACCESS] = fakeRanUe3gpp

		// 斷言：3GPP Access 應該是 CM-CONNECTED
		require.True(t, ue.CmConnect(models.AccessType__3_GPP_ACCESS),
			"3GPP access should be CM-CONNECTED")
		require.False(t, ue.CmIdle(models.AccessType__3_GPP_ACCESS),
			"3GPP access should NOT be CM-IDLE")

		// 斷言：Non-3GPP Access 應該仍然是 CM-IDLE
		require.True(t, ue.CmIdle(models.AccessType_NON_3_GPP_ACCESS),
			"Non-3GPP access should still be CM-IDLE")
		require.False(t, ue.CmConnect(models.AccessType_NON_3_GPP_ACCESS),
			"Non-3GPP access should NOT be CM-CONNECTED")
	})
}

// TestAmfUeCmState_AfterRanUeDetach tests CM state after detaching RanUe
// 測試目標：驗證 RanUe 分離後，AmfUe 狀態恢復為 CM-IDLE
func TestAmfUeCmState_AfterRanUeDetach(t *testing.T) {
	// 準備：建立 AmfUe 並初始化
	ue := &AmfUe{}
	ue.init()

	// 先連接 RanUe
	fakeRanUe := &RanUe{
		RanUeNgapId: 1,
		AmfUeNgapId: 1,
		Log:         logger.NgapLog.WithField("test", "detach"),
	}
	ue.RanUe[models.AccessType__3_GPP_ACCESS] = fakeRanUe

	// 驗證已連接
	require.True(t, ue.CmConnect(models.AccessType__3_GPP_ACCESS),
		"AmfUe should be CM-CONNECTED after attaching RanUe")

	t.Run("Detach RanUe - Return to CM-IDLE", func(t *testing.T) {
		// 移除 RanUe (模擬連線斷開)
		delete(ue.RanUe, models.AccessType__3_GPP_ACCESS)

		// 斷言：現在應該回到 CM-IDLE 狀態
		require.True(t, ue.CmIdle(models.AccessType__3_GPP_ACCESS),
			"AmfUe should return to CM-IDLE state after RanUe is detached")

		// 斷言：不應該是 CM-CONNECTED 狀態
		require.False(t, ue.CmConnect(models.AccessType__3_GPP_ACCESS),
			"AmfUe should NOT be CM-CONNECTED after RanUe is detached")
	})
}

// TestAmfUeCmState_BothAccessTypes tests CM state with both access types connected
// 測試目標：驗證同時連接兩種 Access Type 時的狀態
func TestAmfUeCmState_BothAccessTypes(t *testing.T) {
	// 準備：建立 AmfUe 並初始化
	ue := &AmfUe{}
	ue.init()

	t.Run("Both Access Types Connected", func(t *testing.T) {
		// 連接 3GPP Access
		fakeRanUe3gpp := &RanUe{
			RanUeNgapId: 1,
			AmfUeNgapId: 1,
			Log:         logger.NgapLog.WithField("test", "3gpp"),
		}
		ue.RanUe[models.AccessType__3_GPP_ACCESS] = fakeRanUe3gpp

		// 連接 Non-3GPP Access
		fakeRanUeNon3gpp := &RanUe{
			RanUeNgapId: 2,
			AmfUeNgapId: 2,
			Log:         logger.NgapLog.WithField("test", "non3gpp"),
		}
		ue.RanUe[models.AccessType_NON_3_GPP_ACCESS] = fakeRanUeNon3gpp

		// 斷言：兩種 Access Type 都應該是 CM-CONNECTED
		require.True(t, ue.CmConnect(models.AccessType__3_GPP_ACCESS),
			"3GPP access should be CM-CONNECTED")
		require.True(t, ue.CmConnect(models.AccessType_NON_3_GPP_ACCESS),
			"Non-3GPP access should be CM-CONNECTED")

		// 斷言：兩種 Access Type 都不應該是 CM-IDLE
		require.False(t, ue.CmIdle(models.AccessType__3_GPP_ACCESS),
			"3GPP access should NOT be CM-IDLE")
		require.False(t, ue.CmIdle(models.AccessType_NON_3_GPP_ACCESS),
			"Non-3GPP access should NOT be CM-IDLE")
	})

	t.Run("Detach One Access Type", func(t *testing.T) {
		// 移除 3GPP Access
		delete(ue.RanUe, models.AccessType__3_GPP_ACCESS)

		// 斷言：3GPP Access 應該變回 CM-IDLE
		require.True(t, ue.CmIdle(models.AccessType__3_GPP_ACCESS),
			"3GPP access should return to CM-IDLE")

		// 斷言：Non-3GPP Access 應該仍然是 CM-CONNECTED
		require.True(t, ue.CmConnect(models.AccessType_NON_3_GPP_ACCESS),
			"Non-3GPP access should still be CM-CONNECTED")
	})
}

// TestAmfUeCmState_NilRanUe tests CM state when RanUe is explicitly set to nil
// 測試目標：驗證 RanUe 設為 nil 時的狀態處理
func TestAmfUeCmState_NilRanUe(t *testing.T) {
	// 準備：建立 AmfUe 並初始化
	ue := &AmfUe{}
	ue.init()

	t.Run("RanUe set to nil", func(t *testing.T) {
		// 明確設定 RanUe 為 nil (而不是刪除 key)
		ue.RanUe[models.AccessType__3_GPP_ACCESS] = nil

		// 斷言：應該被視為 CM-IDLE (因為 RanUe 是 nil)
		require.True(t, ue.CmIdle(models.AccessType__3_GPP_ACCESS),
			"AmfUe should be CM-IDLE when RanUe is nil")

		// 斷言：不應該是 CM-CONNECTED
		require.False(t, ue.CmConnect(models.AccessType__3_GPP_ACCESS),
			"AmfUe should NOT be CM-CONNECTED when RanUe is nil")
	})
}

// ==================== 邊緣測試 (Edge Case Tests) ====================

// TestAmfUeCmState_RapidStateChange tests rapid CM state transitions
// 測試目標：測試快速連接/斷開循環，驗證狀態轉換的穩定性
func TestAmfUeCmState_RapidStateChange(t *testing.T) {
	// 準備：建立 AmfUe 並初始化
	ue := &AmfUe{}
	ue.init()

	accessType := models.AccessType__3_GPP_ACCESS
	cycles := 100

	t.Run("100 Rapid Attach-Detach Cycles", func(t *testing.T) {
		// 執行 100 次快速的 Attach-Detach 循環
		for i := 0; i < cycles; i++ {
			// Attach: 建立新的 RanUe 並連接
			fakeRanUe := &RanUe{
				RanUeNgapId: int64(i + 1),
				AmfUeNgapId: int64(i + 1),
				Log:         logger.NgapLog.WithField("cycle", i),
			}
			ue.RanUe[accessType] = fakeRanUe

			// 驗證: 應該是 CM-CONNECTED
			require.True(t, ue.CmConnect(accessType),
				"Cycle %d: Should be CM-CONNECTED after attach", i)
			require.False(t, ue.CmIdle(accessType),
				"Cycle %d: Should NOT be CM-IDLE after attach", i)

			// Detach: 移除 RanUe
			delete(ue.RanUe, accessType)

			// 驗證: 應該是 CM-IDLE
			require.True(t, ue.CmIdle(accessType),
				"Cycle %d: Should be CM-IDLE after detach", i)
			require.False(t, ue.CmConnect(accessType),
				"Cycle %d: Should NOT be CM-CONNECTED after detach", i)
		}

		// 最終驗證: 經過 100 次循環後，狀態仍然正確
		require.True(t, ue.CmIdle(accessType),
			"After %d cycles, AmfUe should be in CM-IDLE state", cycles)

		// 驗證: RanUe map 應該是空的
		_, exists := ue.RanUe[accessType]
		require.False(t, exists,
			"After %d cycles, RanUe should not exist in map", cycles)
	})

	t.Run("Verify No Memory Leak After Rapid Cycles", func(t *testing.T) {
		// 驗證: RanUe map 的大小應該是 0 或只包含其他 AccessType
		count := 0
		for range ue.RanUe {
			count++
		}
		require.Equal(t, 0, count,
			"RanUe map should be empty after all cycles")
	})
}

// TestAmfUeCmState_MultipleRanUeForSameAccessType tests attaching multiple RanUe to same AccessType
// 測試目標：測試同一個 AccessType 被多次 Attach 不同 RanUe 的行為
func TestAmfUeCmState_MultipleRanUeForSameAccessType(t *testing.T) {
	// 準備：建立 AmfUe 並初始化
	ue := &AmfUe{}
	ue.init()

	accessType := models.AccessType__3_GPP_ACCESS

	t.Run("Attach First RanUe", func(t *testing.T) {
		// 第一次 Attach
		ranUe1 := &RanUe{
			RanUeNgapId: 1,
			AmfUeNgapId: 1,
			Log:         logger.NgapLog.WithField("ranUe", "first"),
		}
		ue.RanUe[accessType] = ranUe1

		// 驗證: 應該是 CM-CONNECTED
		require.True(t, ue.CmConnect(accessType))
		
		// 驗證: RanUe 應該是 ranUe1
		require.Equal(t, ranUe1, ue.RanUe[accessType],
			"RanUe should point to first attached RanUe")
	})

	t.Run("Attach Second RanUe Without Detaching First", func(t *testing.T) {
		// 第二次 Attach (不先 Detach，直接覆蓋)
		ranUe2 := &RanUe{
			RanUeNgapId: 2,
			AmfUeNgapId: 2,
			Log:         logger.NgapLog.WithField("ranUe", "second"),
		}
		ue.RanUe[accessType] = ranUe2

		// 驗證: 仍然是 CM-CONNECTED
		require.True(t, ue.CmConnect(accessType),
			"Should still be CM-CONNECTED after replacing RanUe")

		// 驗證: RanUe 應該被覆蓋為 ranUe2
		require.Equal(t, ranUe2, ue.RanUe[accessType],
			"RanUe should be replaced with second RanUe")
		require.Equal(t, int64(2), ue.RanUe[accessType].RanUeNgapId,
			"RanUeNgapId should be 2 (from second RanUe)")
	})

	t.Run("Attach Third RanUe", func(t *testing.T) {
		// 第三次 Attach
		ranUe3 := &RanUe{
			RanUeNgapId: 3,
			AmfUeNgapId: 3,
			Log:         logger.NgapLog.WithField("ranUe", "third"),
		}
		ue.RanUe[accessType] = ranUe3

		// 驗證: 最終應該是最後一次 Attach 的 RanUe
		require.Equal(t, ranUe3, ue.RanUe[accessType])
		require.Equal(t, int64(3), ue.RanUe[accessType].RanUeNgapId)
	})

	t.Run("Final State Check", func(t *testing.T) {
		// 驗證: 狀態仍然是 CM-CONNECTED
		require.True(t, ue.CmConnect(accessType))
		
		// 驗證: 只有一個 RanUe (最後一個)
		require.NotNil(t, ue.RanUe[accessType])
	})
}

// TestAmfUeCmState_CheckAfterAmfUeInit tests CM state check without proper initialization
// 測試目標：測試 AmfUe 未完全初始化時的狀態檢查
func TestAmfUeCmState_CheckAfterAmfUeInit(t *testing.T) {
	t.Run("Check CM State Without Calling init()", func(t *testing.T) {
		// 建立 AmfUe 但不呼叫 init()
		ue := &AmfUe{}
		// 注意: 沒有呼叫 ue.init()

		accessType := models.AccessType__3_GPP_ACCESS

		// 這個測試驗證即使沒有初始化，函數也不應該 panic
		require.NotPanics(t, func() {
			// 嘗試檢查 CM 狀態
			_ = ue.CmIdle(accessType)
			_ = ue.CmConnect(accessType)
		}, "CM state check should not panic even without init()")

		// 驗證: 未初始化的 AmfUe，RanUe map 應該是 nil
		if ue.RanUe == nil {
			t.Log("RanUe map is nil (expected without init)")
		}

		// 如果 RanUe 是 nil，CmConnect 應該回傳 false
		if ue.RanUe == nil {
			require.False(t, ue.CmConnect(accessType),
				"CmConnect should return false when RanUe map is nil")
		}
	})

	t.Run("Check CM State After Partial Init", func(t *testing.T) {
		// 建立 AmfUe 並只初始化 RanUe map (不完整的初始化)
		ue := &AmfUe{
			RanUe: make(map[models.AccessType]*RanUe),
		}

		accessType := models.AccessType__3_GPP_ACCESS

		// 驗證: 部分初始化的情況下，應該是 CM-IDLE
		require.True(t, ue.CmIdle(accessType),
			"Partially initialized AmfUe should be CM-IDLE")
		require.False(t, ue.CmConnect(accessType),
			"Partially initialized AmfUe should NOT be CM-CONNECTED")
	})

	t.Run("Normal Init and Verify", func(t *testing.T) {
		// 建立 AmfUe 並正確初始化
		ue := &AmfUe{}
		ue.init()

		accessType := models.AccessType__3_GPP_ACCESS

		// 驗證: 正確初始化後，應該是 CM-IDLE
		require.True(t, ue.CmIdle(accessType))
		require.False(t, ue.CmConnect(accessType))

		// 驗證: RanUe map 應該已經初始化
		require.NotNil(t, ue.RanUe, "RanUe map should be initialized")
	})
}

// TestAmfUeCmState_ConcurrentStateCheck tests concurrent CM state checks
// 測試目標：測試並發讀取 CM 狀態的安全性
func TestAmfUeCmState_ConcurrentStateCheck(t *testing.T) {
	// 準備：建立 AmfUe 並初始化
	ue := &AmfUe{}
	ue.init()

	accessType := models.AccessType__3_GPP_ACCESS

	// 先連接一個 RanUe
	fakeRanUe := &RanUe{
		RanUeNgapId: 1,
		AmfUeNgapId: 1,
		Log:         logger.NgapLog.WithField("test", "concurrent"),
	}
	ue.RanUe[accessType] = fakeRanUe

	t.Run("100 Concurrent State Checks", func(t *testing.T) {
		// 啟動 100 個 goroutine 同時檢查狀態
		done := make(chan bool, 100)

		for i := 0; i < 100; i++ {
			go func(id int) {
				defer func() {
					done <- true
				}()

				// 重複檢查狀態多次
				for j := 0; j < 10; j++ {
					isIdle := ue.CmIdle(accessType)
					isConnected := ue.CmConnect(accessType)

					// 驗證: 狀態應該一致 (一個為 true，另一個為 false)
					require.NotEqual(t, isIdle, isConnected,
						"Goroutine %d iteration %d: CM-IDLE and CM-CONNECTED should be opposite", id, j)

					// 驗證: 應該是 CM-CONNECTED
					require.True(t, isConnected,
						"Goroutine %d iteration %d: Should be CM-CONNECTED", id, j)
				}
			}(i)
		}

		// 等待所有 goroutine 完成
		for i := 0; i < 100; i++ {
			<-done
		}
	})

	t.Run("Verify Final State", func(t *testing.T) {
		// 驗證: 最終狀態仍然是 CM-CONNECTED
		require.True(t, ue.CmConnect(accessType))
		require.False(t, ue.CmIdle(accessType))
	})
}

// TestAmfUeCmState_AlternatingAccessTypes tests rapidly alternating between access types
// 測試目標：測試在兩種 AccessType 之間快速切換
func TestAmfUeCmState_AlternatingAccessTypes(t *testing.T) {
	// 準備：建立 AmfUe 並初始化
	ue := &AmfUe{}
	ue.init()

	cycles := 50

	t.Run("Alternating Attach Between 3GPP and Non-3GPP", func(t *testing.T) {
		for i := 0; i < cycles; i++ {
			// 偶數循環: Attach 3GPP
			if i%2 == 0 {
				ranUe3gpp := &RanUe{
					RanUeNgapId: int64(i + 1),
					AmfUeNgapId: int64(i + 1),
					Log:         logger.NgapLog.WithField("cycle", i),
				}
				ue.RanUe[models.AccessType__3_GPP_ACCESS] = ranUe3gpp

				// 驗證 3GPP
				require.True(t, ue.CmConnect(models.AccessType__3_GPP_ACCESS),
					"Cycle %d: 3GPP should be CM-CONNECTED", i)

				// 移除之前的 Non-3GPP (如果有)
				delete(ue.RanUe, models.AccessType_NON_3_GPP_ACCESS)
			} else {
				// 奇數循環: Attach Non-3GPP
				ranUeNon3gpp := &RanUe{
					RanUeNgapId: int64(i + 1),
					AmfUeNgapId: int64(i + 1),
					Log:         logger.NgapLog.WithField("cycle", i),
				}
				ue.RanUe[models.AccessType_NON_3_GPP_ACCESS] = ranUeNon3gpp

				// 驗證 Non-3GPP
				require.True(t, ue.CmConnect(models.AccessType_NON_3_GPP_ACCESS),
					"Cycle %d: Non-3GPP should be CM-CONNECTED", i)

				// 移除之前的 3GPP (如果有)
				delete(ue.RanUe, models.AccessType__3_GPP_ACCESS)
			}
		}
	})

	t.Run("Final State Verification", func(t *testing.T) {
		// cycles = 50, 最後一個循環是 i=49 (奇數)
		// 奇數循環會 Attach Non-3GPP, 所以最終 Non-3GPP 應該是 CONNECTED
		lastCycle := cycles - 1
		
		if lastCycle%2 == 0 {
			// 最後是偶數循環 - 3GPP CONNECTED
			require.True(t, ue.CmConnect(models.AccessType__3_GPP_ACCESS),
				"Final state: 3GPP should be CM-CONNECTED (last cycle was even)")
			require.True(t, ue.CmIdle(models.AccessType_NON_3_GPP_ACCESS),
				"Final state: Non-3GPP should be CM-IDLE")
		} else {
			// 最後是奇數循環 - Non-3GPP CONNECTED
			require.True(t, ue.CmIdle(models.AccessType__3_GPP_ACCESS),
				"Final state: 3GPP should be CM-IDLE")
			require.True(t, ue.CmConnect(models.AccessType_NON_3_GPP_ACCESS),
				"Final state: Non-3GPP should be CM-CONNECTED (last cycle was odd)")
		}
		
		t.Logf("Last cycle index: %d (even=%v)", lastCycle, lastCycle%2 == 0)
	})
}
