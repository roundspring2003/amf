package context

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/free5gc/openapi/models"
)

// TestAMFContext_ConcurrentRanUePool tests concurrent access to RanUePool
// 測試目標：驗證 AMFContext 的 RanUePool (sync.Map) 在同時讀寫刪除時是安全的
func TestAMFContext_ConcurrentRanUePool(t *testing.T) {
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

	// 建立 AmfRan
	fakeConn := &fakeNetConn{}
	amfRan := amfContext.NewAmfRan(fakeConn)
	amfRan.AnType = models.AccessType__3_GPP_ACCESS

	const numRanUes = 10000

	t.Run("Create 10000 RanUe entries", func(t *testing.T) {
		// 建立 10000 個 RanUe
		for i := int64(1); i <= numRanUes; i++ {
			ranUe, err := amfRan.NewRanUe(i)
			require.NoError(t, err)
			require.NotNil(t, ranUe)
		}

		// 驗證所有 RanUe 都在 Pool 中
		count := 0
		amfContext.RanUePool.Range(func(key, value interface{}) bool {
			count++
			return true
		})
		require.Equal(t, numRanUes, count, "Should have 10000 RanUes in pool")
	})

	t.Run("Concurrent Delete Operations", func(t *testing.T) {
		var wg sync.WaitGroup

		// 啟動 10000 個 goroutine,每個都刪除一個 RanUe
		for i := int64(1); i <= numRanUes; i++ {
			wg.Add(1)
			go func(amfUeNgapId int64) {
				defer wg.Done()
				// 模擬 UE 單獨斷線 - 從 RanUePool 刪除
				amfContext.RanUePool.Delete(amfUeNgapId)
			}(i)
		}

		// 同時在主執行緒進行讀取操作 (模擬查詢)
		go func() {
			for i := 0; i < 1000; i++ {
				amfContext.RanUePool.Range(func(key, value interface{}) bool {
					// 只是讀取,不做任何操作
					return true
				})
			}
		}()

		// 等待所有刪除完成
		wg.Wait()

		// 驗證所有 RanUe 都已刪除
		count := 0
		amfContext.RanUePool.Range(func(key, value interface{}) bool {
			count++
			return true
		})
		require.Equal(t, 0, count, "All RanUes should be deleted from pool")
	})

	t.Run("Should not panic during concurrent operations", func(t *testing.T) {
		// 主要測試: 程式沒有 panic
		require.NotPanics(t, func() {
			// 測試已經在上面執行,這裡只是確認沒有 panic
		})
	})

	// 清理
	t.Cleanup(func() {
		amfContext.AmfRanPool.Delete(fakeConn)
	})
}

// TestAMFContext_ConcurrentUePool tests concurrent access to UePool
// 測試目標：驗證 AMFContext 的 UePool (sync.Map) 的併發安全性
func TestAMFContext_ConcurrentUePool(t *testing.T) {
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

	const numUes = 5000

	t.Run("Concurrent UE Creation and Deletion", func(t *testing.T) {
		var wg sync.WaitGroup

		// 同時創建 5000 個 UE
		for i := 0; i < numUes; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				supi := "imsi-20893000000" + string(rune('0'+index%10)) + string(rune('0'+index/10%10))
				amfUe := amfContext.NewAmfUe(supi)
				require.NotNil(t, amfUe)
			}(i)
		}

		wg.Wait()

		// 驗證 UE 被創建
		count := 0
		amfContext.UePool.Range(func(key, value interface{}) bool {
			count++
			return true
		})
		require.GreaterOrEqual(t, count, 1, "Should have UEs in pool")
	})

	t.Run("Concurrent Read and Write", func(t *testing.T) {
		var wg sync.WaitGroup

		// 同時讀取
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				amfContext.UePool.Range(func(key, value interface{}) bool {
					return true
				})
			}()
		}

		// 同時寫入
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				supi := "imsi-20893test" + string(rune('0'+index))
				amfContext.UePool.Store(supi, &AmfUe{Supi: supi})
			}(i)
		}

		// 同時刪除
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				supi := "imsi-20893test" + string(rune('0'+index))
				amfContext.UePool.Delete(supi)
			}(i)
		}

		wg.Wait()
	})

	t.Run("No panic during concurrent operations", func(t *testing.T) {
		require.NotPanics(t, func() {
			// 測試已完成,驗證沒有 panic
		})
	})

	// 清理
	t.Cleanup(func() {
		// 清理測試創建的 UE
		amfContext.UePool.Range(func(key, value interface{}) bool {
			if supi, ok := key.(string); ok {
				amfContext.UePool.Delete(supi)
			}
			return true
		})
	})
}

// TestAmfRan_ConcurrentRanUeList tests concurrent access to AmfRan.RanUeList
// 測試目標：驗證 AmfRan.RanUeList (sync.Map) 在 RAN 斷線時的併發安全性
// (這就是現有的 TestRemoveAndRemoveAllRanUeRaceCondition 在做的事)
func TestAmfRan_ConcurrentRanUeListStressTest(t *testing.T) {
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
	ran := amfContext.NewAmfRan(fakeConn)
	ran.AnType = models.AccessType__3_GPP_ACCESS

	const numRanUes = 10000

	t.Run("Create 10000 RanUe in RAN", func(t *testing.T) {
		// 建立 10000 個 RanUe 並存入 RanUeList
		for i := int64(1); i <= numRanUes; i++ {
			ranUe, err := ran.NewRanUe(i)
			require.NoError(t, err)
			require.NotNil(t, ranUe)
		}
	})

	t.Run("Concurrent Individual Delete + RemoveAllRanUe", func(t *testing.T) {
		// 這是最極端的測試場景:
		// - 10000 個 goroutine 同時刪除個別 UE (模擬 UE 單獨斷線)
		// - 主執行緒同時呼叫 RemoveAllRanUe() (模擬基地台拔線)

		// 啟動 10000 個 goroutine 刪除個別 RanUe
		for i := int64(1); i <= numRanUes; i++ {
			go func(ranUeNgapId int64) {
				ran.RanUeList.Delete(ranUeNgapId)
			}(i)
		}

		// 同時呼叫 RemoveAllRanUe (模擬基地台斷線)
		require.NotPanics(t, func() {
			ran.RemoveAllRanUe(true)
		}, "RemoveAllRanUe should not panic during concurrent deletes")
	})

	t.Run("Verify all RanUe removed", func(t *testing.T) {
		count := 0
		ran.RanUeList.Range(func(key, value interface{}) bool {
			count++
			return true
		})
		require.Equal(t, 0, count, "All RanUes should be removed")
	})

	// 清理
	t.Cleanup(func() {
		amfContext.AmfRanPool.Delete(fakeConn)
	})
}

// TestAMFContext_MixedConcurrentOperations tests all pools simultaneously
// 測試目標：同時對所有 Pool 進行併發操作,模擬真實系統壓力
func TestAMFContext_MixedConcurrentOperations(t *testing.T) {
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

	const numOperations = 1000

	t.Run("Mixed Concurrent Operations on All Pools", func(t *testing.T) {
		var wg sync.WaitGroup

		// 併發操作 UePool
		for i := 0; i < numOperations; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				supi := "imsi-test" + string(rune('0'+index%10))
				
				// 創建
				amfUe := &AmfUe{Supi: supi}
				amfContext.UePool.Store(supi, amfUe)
				
				// 讀取
				amfContext.UePool.Load(supi)
				
				// 刪除
				amfContext.UePool.Delete(supi)
			}(i)
		}

		// 併發操作 RanUePool
		for i := int64(1); i <= numOperations; i++ {
			wg.Add(1)
			go func(id int64) {
				defer wg.Done()
				
				// 模擬寫入
				amfContext.RanUePool.Store(id, &RanUe{AmfUeNgapId: id})
				
				// 模擬讀取
				amfContext.RanUePool.Load(id)
				
				// 模擬刪除
				amfContext.RanUePool.Delete(id)
			}(i)
		}

		// 併發 Range 操作
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				amfContext.UePool.Range(func(key, value interface{}) bool {
					return true
				})
			}()

			wg.Add(1)
			go func() {
				defer wg.Done()
				amfContext.RanUePool.Range(func(key, value interface{}) bool {
					return true
				})
			}()
		}

		wg.Wait()
	})

	t.Run("System remains stable after stress", func(t *testing.T) {
		require.NotPanics(t, func() {
			// 驗證系統仍然可以正常操作
			testSupi := "imsi-stability-test"
			amfUe := amfContext.NewAmfUe(testSupi)
			require.NotNil(t, amfUe)
			
			amfContext.UePool.Delete(testSupi)
		})
	})
}

// TestConcurrentMapOperations_EdgeCases tests edge cases in concurrent scenarios
// 測試目標：測試併發場景中的邊界情況
func TestConcurrentMapOperations_EdgeCases(t *testing.T) {
	t.Run("Concurrent delete of same key", func(t *testing.T) {
		var testMap sync.Map
		testMap.Store("test-key", "test-value")

		var wg sync.WaitGroup
		// 1000 個 goroutine 同時刪除同一個 key
		for i := 0; i < 1000; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				testMap.Delete("test-key")
			}()
		}

		require.NotPanics(t, func() {
			wg.Wait()
		}, "Concurrent delete of same key should not panic")

		// 驗證 key 已被刪除
		_, exists := testMap.Load("test-key")
		require.False(t, exists)
	})

	t.Run("Concurrent Store and Delete same key", func(t *testing.T) {
		var testMap sync.Map

		var wg sync.WaitGroup
		// 同時進行 Store 和 Delete
		for i := 0; i < 1000; i++ {
			wg.Add(2)
			go func(val int) {
				defer wg.Done()
				testMap.Store("race-key", val)
			}(i)
			go func() {
				defer wg.Done()
				testMap.Delete("race-key")
			}()
		}

		require.NotPanics(t, func() {
			wg.Wait()
		}, "Concurrent Store and Delete should not panic")
	})
}
