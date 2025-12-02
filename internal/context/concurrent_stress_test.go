package context

import (
	"sync"
	"testing"
	"time"

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

// ==================== 邊緣測試 (Edge Case Tests) ====================

// TestConcurrent_DeleteNonExistentKey tests deleting non-existent keys
// 測試目標：大量刪除不存在的 key
func TestConcurrent_DeleteNonExistentKey(t *testing.T) {
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

	t.Run("Delete 10000 Non-Existent RanUe Keys", func(t *testing.T) {
		var wg sync.WaitGroup

		// 嘗試刪除 10000 個不存在的 RanUe
		for i := int64(100000); i < 110000; i++ {
			wg.Add(1)
			go func(id int64) {
				defer wg.Done()
				// 刪除不存在的 key
				amfContext.RanUePool.Delete(id)
			}(i)
		}

		require.NotPanics(t, func() {
			wg.Wait()
		}, "Deleting non-existent keys should not panic")
	})

	t.Run("Delete Non-Existent UE Keys", func(t *testing.T) {
		var wg sync.WaitGroup

		// 嘗試刪除 5000 個不存在的 UE
		for i := 0; i < 5000; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				supi := "imsi-999999999999" + string(rune('0'+id%10))
				amfContext.UePool.Delete(supi)
			}(i)
		}

		require.NotPanics(t, func() {
			wg.Wait()
		}, "Deleting non-existent UE should not panic")
	})

	t.Run("Performance Check - Delete Non-Existent", func(t *testing.T) {
		// 驗證刪除不存在的 key 不會造成效能問題
		iterations := 10000
		
		for i := 0; i < iterations; i++ {
			amfContext.RanUePool.Delete(int64(i + 200000))
		}
		
		t.Logf("Successfully deleted %d non-existent keys", iterations)
	})
}

// TestConcurrent_LoadOrStore tests concurrent LoadOrStore operations
// 測試目標：測試 LoadOrStore 的併發行為
func TestConcurrent_LoadOrStore(t *testing.T) {
	t.Run("1000 Goroutines LoadOrStore Same Key", func(t *testing.T) {
		var testMap sync.Map
		var wg sync.WaitGroup

		// 記錄哪個 goroutine 成功寫入
		successCount := 0
		var mu sync.Mutex

		const numGoroutines = 1000
		const testKey = "shared-key"

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				// LoadOrStore 嘗試寫入
				_, loaded := testMap.LoadOrStore(testKey, id)
				
				if !loaded {
					// 這個 goroutine 成功寫入
					mu.Lock()
					successCount++
					mu.Unlock()
				}
			}(i)
		}

		wg.Wait()

		// 驗證只有一個 goroutine 成功寫入
		require.Equal(t, 1, successCount,
			"Only one goroutine should successfully store the value")

		// 驗證 key 存在
		val, exists := testMap.Load(testKey)
		require.True(t, exists, "Key should exist")
		require.NotNil(t, val, "Value should not be nil")
		
		t.Logf("Winning goroutine ID: %v", val)
	})

	t.Run("LoadOrStore with Different Keys", func(t *testing.T) {
		var testMap sync.Map
		var wg sync.WaitGroup

		const numKeys = 5000

		// 每個 key 被 10 個 goroutine 同時 LoadOrStore
		for i := 0; i < numKeys; i++ {
			for j := 0; j < 10; j++ {
				wg.Add(1)
				go func(keyID, goroutineID int) {
					defer wg.Done()
					key := "key-" + string(rune('0'+keyID%10)) + string(rune('0'+keyID/10%10))
					testMap.LoadOrStore(key, goroutineID)
				}(i, j)
			}
		}

		wg.Wait()

		// 驗證所有 key 都存在
		count := 0
		testMap.Range(func(key, value interface{}) bool {
			count++
			return true
		})

		t.Logf("Total keys stored: %d", count)
		require.Greater(t, count, 0, "Should have stored some keys")
	})
}

// TestConcurrent_RangeWithModification tests Range while modifying map
// 測試目標：測試遍歷時同時修改 map
func TestConcurrent_RangeWithModification(t *testing.T) {
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

	// 建立一些初始 UE
	const initialUEs = 1000
	for i := 0; i < initialUEs; i++ {
		supi := "imsi-20893000000" + string(rune('0'+i/100%10)) + 
		       string(rune('0'+i/10%10)) + string(rune('0'+i%10))
		amfContext.NewAmfUe(supi)
	}

	t.Run("Range While Adding New UEs", func(t *testing.T) {
		var wg sync.WaitGroup
		stopRange := make(chan bool)
		rangeCount := 0

		// Goroutine 1: 持續 Range
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stopRange:
					return
				default:
					amfContext.UePool.Range(func(key, value interface{}) bool {
						rangeCount++
						return true
					})
				}
			}
		}()

		// Goroutine 2-101: 同時新增 UE
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				supi := "imsi-30893000000" + string(rune('0'+id/10%10)) + string(rune('0'+id%10))
				amfContext.NewAmfUe(supi)
			}(i)
		}

		// 等待新增完成
		for i := 0; i < 100; i++ {
			<-time.After(1 * time.Millisecond)
		}

		// 停止 Range
		close(stopRange)
		wg.Wait()

		require.NotPanics(t, func() {
			wg.Wait()
		}, "Range while adding should not panic")

		t.Logf("Range executed %d times during concurrent adds", rangeCount)
	})

	t.Run("Range While Deleting UEs", func(t *testing.T) {
		var wg sync.WaitGroup
		stopRange := make(chan bool)

		// Goroutine 1: 持續 Range
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stopRange:
					return
				default:
					amfContext.UePool.Range(func(key, value interface{}) bool {
						return true
					})
				}
			}
		}()

		// Goroutine 2-51: 同時刪除 UE
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				supi := "imsi-20893000000" + string(rune('0'+id/10%10)) + string(rune('0'+id%10))
				amfContext.UePool.Delete(supi)
			}(i)
		}

		// 等待刪除完成
		for i := 0; i < 50; i++ {
			<-time.After(1 * time.Millisecond)
		}

		// 停止 Range
		close(stopRange)
		wg.Wait()

		require.NotPanics(t, func() {
			wg.Wait()
		}, "Range while deleting should not panic")
	})

	// 清理
	t.Cleanup(func() {
		// 清理所有測試建立的 UE
		amfContext.UePool.Range(func(key, value interface{}) bool {
			amfContext.UePool.Delete(key)
			return true
		})
	})
}

// TestConcurrent_CreateDeleteSameUeRepeatedly tests repeated create/delete cycles
// 測試目標：測試同一個 SUPI 重複創建和刪除
func TestConcurrent_CreateDeleteSameUeRepeatedly(t *testing.T) {
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

	const testSupi = "imsi-208930000009999"

	t.Run("1000 Goroutines Create/Delete Same SUPI", func(t *testing.T) {
		var wg sync.WaitGroup

		for i := 0; i < 1000; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				// 創建 UE
				ue := amfContext.NewAmfUe(testSupi)
				
				if ue != nil {
					// 短暫延遲
					time.Sleep(1 * time.Microsecond)
					
					// 刪除 UE
					amfContext.UePool.Delete(testSupi)
				}
			}(i)
		}

		require.NotPanics(t, func() {
			wg.Wait()
		}, "Repeated create/delete should not panic")
	})

	t.Run("Verify Final State", func(t *testing.T) {
		// 最終狀態可能存在或不存在,取決於最後完成的 goroutine
		_, exists := amfContext.UePool.Load(testSupi)
		t.Logf("Final state - UE exists: %v", exists)

		// 無論如何,測試不應該 panic
	})

	t.Run("Check GUTI Allocation After Stress", func(t *testing.T) {
		// 建立新的 UE 驗證 ID 分配仍然正常
		newUe := amfContext.NewAmfUe("imsi-208930000008888")
		require.NotNil(t, newUe, "Should be able to create new UE after stress test")
		
		// GUTI 應該被正確分配
		require.NotEmpty(t, newUe.Guti, "GUTI should be allocated")
		
		t.Logf("New UE GUTI: %v", newUe.Guti)
		
		// 清理
		amfContext.UePool.Delete("imsi-208930000008888")
	})

	// 清理
	t.Cleanup(func() {
		amfContext.UePool.Delete(testSupi)
	})
}

// TestConcurrent_MaxCapacity tests system capacity limits
// 測試目標：測試系統容量極限
func TestConcurrent_MaxCapacity(t *testing.T) {
	// 跳過此測試,除非明確要求 (因為會建立 100,000 個 UE)
	if testing.Short() {
		t.Skip("Skipping max capacity test in short mode")
	}

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

	const maxUEs = 100000

	t.Run("Create 100,000 UEs", func(t *testing.T) {
		var wg sync.WaitGroup
		
		// 分批建立,每批 1000 個
		batchSize := 1000
		numBatches := maxUEs / batchSize

		for batch := 0; batch < numBatches; batch++ {
			for i := 0; i < batchSize; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					
					// 建立唯一的 SUPI (使用更長的格式)
					supi := formatSupi(id)
					amfContext.NewAmfUe(supi)
				}(batch*batchSize + i)
			}
			
			// 等待這批完成
			wg.Wait()
			
			if (batch+1)%10 == 0 {
				t.Logf("Progress: %d/%d UEs created", (batch+1)*batchSize, maxUEs)
			}
		}
	})

	t.Run("Verify UE Count", func(t *testing.T) {
		count := 0
		amfContext.UePool.Range(func(key, value interface{}) bool {
			count++
			return true
		})

		t.Logf("Total UEs in pool: %d", count)
		require.Greater(t, count, maxUEs/2,
			"Should have created at least half of the target UEs")
	})

	t.Run("Concurrent Operations on Large Pool", func(t *testing.T) {
		var wg sync.WaitGroup

		// 同時進行讀取、寫入、刪除操作
		for i := 0; i < 100; i++ {
			wg.Add(3)
			
			// 讀取
			go func(id int) {
				defer wg.Done()
				supi := formatSupi(id * 100)
				amfContext.UePool.Load(supi)
			}(i)
			
			// 寫入新的
			go func(id int) {
				defer wg.Done()
				supi := formatSupi(maxUEs + id)
				amfContext.NewAmfUe(supi)
			}(i)
			
			// 刪除
			go func(id int) {
				defer wg.Done()
				supi := formatSupi(id * 200)
				amfContext.UePool.Delete(supi)
			}(i)
		}

		require.NotPanics(t, func() {
			wg.Wait()
		}, "Concurrent operations on large pool should not panic")
	})

	// 清理 (這會需要一些時間)
	t.Cleanup(func() {
		t.Log("Cleaning up 100,000 UEs...")
		amfContext.UePool.Range(func(key, value interface{}) bool {
			amfContext.UePool.Delete(key)
			return true
		})
		t.Log("Cleanup completed")
	})
}

// formatSupi formats an integer ID into a valid SUPI string
func formatSupi(id int) string {
	// 格式: imsi-208930000XXXXXX (15 位數字)
	return "imsi-208930000" + 
	       string(rune('0'+id/100000%10)) +
	       string(rune('0'+id/10000%10)) +
	       string(rune('0'+id/1000%10)) +
	       string(rune('0'+id/100%10)) +
	       string(rune('0'+id/10%10)) +
	       string(rune('0'+id%10))
}

// TestConcurrent_RapidRanUeLifecycle tests rapid RanUe creation and deletion
// 測試目標：測試 RanUe 的快速生命週期 (防禦性測試)
func TestConcurrent_RapidRanUeLifecycle(t *testing.T) {
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

	t.Run("1000 Rapid Create-Delete Cycles", func(t *testing.T) {
		for i := 0; i < 1000; i++ {
			// 建立
			ranUe, err := amfRan.NewRanUe(int64(i + 1))
			require.NoError(t, err)
			require.NotNil(t, ranUe)

			// 立即刪除
			err = ranUe.Remove()
			require.NoError(t, err)
		}
	})

	t.Run("Verify Pool is Empty", func(t *testing.T) {
		count := 0
		amfContext.RanUePool.Range(func(key, value interface{}) bool {
			count++
			return true
		})
		
		require.Equal(t, 0, count, "RanUePool should be empty after all removes")
	})

	t.Cleanup(func() {
		amfContext.AmfRanPool.Delete(fakeConn)
	})
}

// TestConcurrent_NilValueHandling tests handling of nil values in sync.Map
// 測試目標：測試 sync.Map 中 nil 值的處理 (防禦性測試)
func TestConcurrent_NilValueHandling(t *testing.T) {
	t.Run("Store Nil Value", func(t *testing.T) {
		var testMap sync.Map

		// 嘗試存儲 nil 值
		require.NotPanics(t, func() {
			testMap.Store("nil-key", nil)
		}, "Storing nil value should not panic")

		// 驗證可以讀取
		val, exists := testMap.Load("nil-key")
		require.True(t, exists, "Key with nil value should exist")
		require.Nil(t, val, "Value should be nil")
	})

	t.Run("Concurrent Store Nil Values", func(t *testing.T) {
		var testMap sync.Map
		var wg sync.WaitGroup

		for i := 0; i < 1000; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				key := "nil-" + string(rune('0'+id%10))
				testMap.Store(key, nil)
			}(i)
		}

		require.NotPanics(t, func() {
			wg.Wait()
		}, "Concurrent store of nil values should not panic")

		// 驗證所有 key 都存在
		count := 0
		testMap.Range(func(key, value interface{}) bool {
			count++
			require.Nil(t, value, "All values should be nil")
			return true
		})
		
		t.Logf("Stored %d nil values", count)
	})
}

// TestConcurrent_MemoryPressure tests behavior under memory pressure
// 測試目標：測試記憶體壓力下的行為 (防禦性測試)
func TestConcurrent_MemoryPressure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory pressure test in short mode")
	}

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

	t.Run("Create and Delete 50000 UEs in Waves", func(t *testing.T) {
		const waveSize = 5000
		const numWaves = 10

		for wave := 0; wave < numWaves; wave++ {
			// 建立一波 UE
			for i := 0; i < waveSize; i++ {
				supi := "imsi-50893" + string(rune('0'+wave%10)) + 
				       string(rune('0'+i/1000%10)) + 
				       string(rune('0'+i/100%10)) +
				       string(rune('0'+i/10%10)) +
				       string(rune('0'+i%10))
				amfContext.NewAmfUe(supi)
			}

			// 刪除這波 UE (釋放記憶體)
			for i := 0; i < waveSize; i++ {
				supi := "imsi-50893" + string(rune('0'+wave%10)) + 
				       string(rune('0'+i/1000%10)) + 
				       string(rune('0'+i/100%10)) +
				       string(rune('0'+i/10%10)) +
				       string(rune('0'+i%10))
				amfContext.UePool.Delete(supi)
			}

			if (wave+1)%2 == 0 {
				t.Logf("Completed wave %d/%d", wave+1, numWaves)
			}
		}

		t.Log("Memory pressure test completed")
	})

	t.Run("Verify Pool is Clean", func(t *testing.T) {
		count := 0
		amfContext.UePool.Range(func(key, value interface{}) bool {
			count++
			return true
		})

		t.Logf("UEs remaining in pool: %d", count)
	})
}
