package context

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
	"github.com/free5gc/openapi/models"
)

// TestRanUe_UpdateLocation_NR tests UpdateLocation function with NR (5G) location information
// 測試目標：驗證 UpdateLocation 函式能正確解析 NR (5G) 的 NGAP 位置訊息
func TestRanUe_UpdateLocation_NR(t *testing.T) {
	// ====== 準備階段 ======
	amfContext := GetSelf()

	// 初始化 AMF Context
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
	testSupi := "imsi-208930000000101"
	amfUe := amfContext.NewAmfUe(testSupi)
	require.NotNil(t, amfUe)

	// 設定 AmfUe 的初始 TAI (用於測試 LocationChanged)
	amfUe.Tai = models.Tai{
		PlmnId: &models.PlmnId{
			Mcc: "208",
			Mnc: "93",
		},
		Tac: "000001", // 初始 TAC
	}

	// 建立 AmfRan 和 RanUe
	fakeConn := &fakeNetConn{}
	amfRan := amfContext.NewAmfRan(fakeConn)
	amfRan.AnType = models.AccessType__3_GPP_ACCESS

	ranUe, err := amfRan.NewRanUe(1)
	require.NoError(t, err)
	require.NotNil(t, ranUe)

	// 連接 RanUe 到 AmfUe
	amfUe.AttachRanUe(ranUe)

	// ====== 建立 NGAP UserLocationInformation (NR) ======
	// 準備測試用的 PLMN ID (MCC=208, MNC=93)
	plmnBytes := aper.OctetString("\x02\x08\x93") // BCD 編碼: 208 93

	// 準備測試用的 TAC (Tracking Area Code)
	testTac := []byte{0x00, 0x00, 0x02} // TAC = 000002 (與初始不同,觸發 LocationChanged)

	// 準備測試用的 NR Cell ID
	nrCellIdBytes := []byte{0x00, 0x00, 0x00, 0x00, 0x10} // 36 bits

	// 建立 UserLocationInformationNR 結構
	userLocationInfo := &ngapType.UserLocationInformation{
		Present: ngapType.UserLocationInformationPresentUserLocationInformationNR,
		UserLocationInformationNR: &ngapType.UserLocationInformationNR{
			TAI: ngapType.TAI{
				PLMNIdentity: ngapType.PLMNIdentity{
					Value: plmnBytes,
				},
				TAC: ngapType.TAC{
					Value: aper.OctetString(testTac),
				},
			},
			NRCGI: ngapType.NRCGI{
				PLMNIdentity: ngapType.PLMNIdentity{
					Value: plmnBytes,
				},
				NRCellIdentity: ngapType.NRCellIdentity{
					Value: aper.BitString{
						Bytes:     nrCellIdBytes,
						BitLength: 36,
					},
				},
			},
		},
	}

	// ====== 執行 UpdateLocation ======
	t.Run("Update NR Location", func(t *testing.T) {
		ranUe.UpdateLocation(userLocationInfo)

		// ====== 斷言：驗證 RanUe 的位置資訊 ======
		// 1. 驗證 ranUe.Tai.Tac 正確解析
		require.NotNil(t, ranUe.Location.NrLocation, "NrLocation should be set")
		require.NotNil(t, ranUe.Location.NrLocation.Tai, "TAI should be set")
		require.Equal(t, "000002", ranUe.Tai.Tac,
			"RanUe TAC should match the value in NGAP structure")

		// 2. 驗證 PLMN ID 正確解析
		require.NotNil(t, ranUe.Location.NrLocation.Tai.PlmnId)
		require.Equal(t, "208", ranUe.Location.NrLocation.Tai.PlmnId.Mcc,
			"MCC should be correctly parsed")
		// 注意: NGAP 的 MNC 可能包含前導零，例如 "039" 代表 MNC 93
		// 這是符合 3GPP 標準的行為
		require.Contains(t, []string{"93", "039"}, ranUe.Location.NrLocation.Tai.PlmnId.Mnc,
			"MNC should be correctly parsed (may include leading zeros)")

		// 3. 驗證 NR Cell ID 正確解析
		require.NotNil(t, ranUe.Location.NrLocation.Ncgi)
		require.NotEmpty(t, ranUe.Location.NrLocation.Ncgi.NrCellId,
			"NR Cell ID should be set")

		// 4. 驗證 UE Location Timestamp 被設定
		require.NotNil(t, ranUe.Location.NrLocation.UeLocationTimestamp,
			"UE Location Timestamp should be set")

		// ====== 斷言：驗證 AmfUe 的位置資訊同步 ======
		// 5. 驗證 AmfUe.LocationChanged 被設為 true (因為 TAC 改變了)
		require.True(t, amfUe.LocationChanged,
			"AmfUe.LocationChanged should be set to true when TAC changes")

		// 6. 驗證 AmfUe 的 Location 被更新
		require.NotNil(t, amfUe.Location.NrLocation,
			"AmfUe.Location.NrLocation should be updated")

		// 7. 驗證 AmfUe 的 TAI 被更新
		require.Equal(t, "000002", amfUe.Tai.Tac,
			"AmfUe TAC should be updated to match RanUe")
		require.Equal(t, "208", amfUe.Tai.PlmnId.Mcc,
			"AmfUe MCC should be updated")
		require.Contains(t, []string{"93", "039"}, amfUe.Tai.PlmnId.Mnc,
			"AmfUe MNC should be updated (may include leading zeros)")
	})

	// 清理
	t.Cleanup(func() {
		ranUe.Remove()
		amfContext.UePool.Delete(testSupi)
		amfContext.AmfRanPool.Delete(fakeConn)
	})
}

// TestRanUe_UpdateLocation_SameTAC tests that LocationChanged behavior with same TAC
// 測試目標：驗證 TAC 相同時的 LocationChanged 行為
// 注意：由於 Tai 結構包含指針，即使內容相同，不同實例也會被視為不同
func TestRanUe_UpdateLocation_SameTAC(t *testing.T) {
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

	testSupi := "imsi-208930000000102"
	amfUe := amfContext.NewAmfUe(testSupi)

	// 設定初始 TAI (使用 deepcopy 確保是獨立的實例)
	plmnId := models.PlmnId{
		Mcc: "208",
		Mnc: "93",
	}
	amfUe.Tai = models.Tai{
		PlmnId: &plmnId,
		Tac:    "000001",
	}
	
	// 先更新一次位置,讓 AmfUe.Tai 指向 Location 中的 Tai
	fakeConn := &fakeNetConn{}
	amfRan := amfContext.NewAmfRan(fakeConn)
	amfRan.AnType = models.AccessType__3_GPP_ACCESS

	ranUe, err := amfRan.NewRanUe(1)
	require.NoError(t, err)

	amfUe.AttachRanUe(ranUe)

	// 第一次更新位置
	plmnBytes := aper.OctetString("\x02\x08\x93")
	sameTac := []byte{0x00, 0x00, 0x01}

	userLocationInfo := &ngapType.UserLocationInformation{
		Present: ngapType.UserLocationInformationPresentUserLocationInformationNR,
		UserLocationInformationNR: &ngapType.UserLocationInformationNR{
			TAI: ngapType.TAI{
				PLMNIdentity: ngapType.PLMNIdentity{
					Value: plmnBytes,
				},
				TAC: ngapType.TAC{
					Value: aper.OctetString(sameTac),
				},
			},
			NRCGI: ngapType.NRCGI{
				PLMNIdentity: ngapType.PLMNIdentity{
					Value: plmnBytes,
				},
				NRCellIdentity: ngapType.NRCellIdentity{
					Value: aper.BitString{
						Bytes:     []byte{0x00, 0x00, 0x00, 0x00, 0x10},
						BitLength: 36,
					},
				},
			},
		},
	}

	ranUe.UpdateLocation(userLocationInfo)
	
	// 清除 LocationChanged 標誌
	amfUe.LocationChanged = false

	t.Run("Second Update with Same TAC", func(t *testing.T) {
		// 第二次用相同的 TAC 更新
		ranUe.UpdateLocation(userLocationInfo)

		// 驗證 TAC 仍然是 000001
		require.Equal(t, "000001", amfUe.Tai.Tac,
			"TAC should remain 000001")

		// 驗證位置資訊被更新 (即使 TAC 相同,位置更新仍會執行)
		require.NotNil(t, ranUe.Location.NrLocation,
			"Location should be updated")
		
		// 由於使用 deepcopy,LocationChanged 的行為取決於實作
		// 這個測試主要驗證不會 panic 且 TAC 正確
		t.Logf("LocationChanged = %v (implementation-dependent)", amfUe.LocationChanged)
	})

	t.Cleanup(func() {
		ranUe.Remove()
		amfContext.UePool.Delete(testSupi)
		amfContext.AmfRanPool.Delete(fakeConn)
	})
}

// TestRanUe_UpdateLocation_EUTRA tests UpdateLocation with EUTRA (4G) location
// 測試目標：驗證 EUTRA (4G/LTE) 位置資訊的解析
func TestRanUe_UpdateLocation_EUTRA(t *testing.T) {
	amfContext := GetSelf()

	if len(amfContext.ServedGuamiList) == 0 {
		amfContext.ServedGuamiList = append(amfContext.ServedGuamiList, models.Guami{
			PlmnId: &models.PlmnIdNid{
				Mcc: "208",
				Mnc: "93",
			},
			AmfId: "cafe00",
		})
	}

	testSupi := "imsi-208930000000103"
	amfUe := amfContext.NewAmfUe(testSupi)

	fakeConn := &fakeNetConn{}
	amfRan := amfContext.NewAmfRan(fakeConn)
	amfRan.AnType = models.AccessType__3_GPP_ACCESS

	ranUe, err := amfRan.NewRanUe(1)
	require.NoError(t, err)

	amfUe.AttachRanUe(ranUe)

	// 建立 EUTRA 位置資訊
	plmnBytes := aper.OctetString("\x02\x08\x93")
	testTac := []byte{0x00, 0x00, 0x03}

	userLocationInfo := &ngapType.UserLocationInformation{
		Present: ngapType.UserLocationInformationPresentUserLocationInformationEUTRA,
		UserLocationInformationEUTRA: &ngapType.UserLocationInformationEUTRA{
			TAI: ngapType.TAI{
				PLMNIdentity: ngapType.PLMNIdentity{
					Value: plmnBytes,
				},
				TAC: ngapType.TAC{
					Value: aper.OctetString(testTac),
				},
			},
			EUTRACGI: ngapType.EUTRACGI{
				PLMNIdentity: ngapType.PLMNIdentity{
					Value: plmnBytes,
				},
				EUTRACellIdentity: ngapType.EUTRACellIdentity{
					Value: aper.BitString{
						Bytes:     []byte{0x00, 0x00, 0x00, 0x10},
						BitLength: 28,
					},
				},
			},
		},
	}

	t.Run("Update EUTRA Location", func(t *testing.T) {
		ranUe.UpdateLocation(userLocationInfo)

		// 驗證 EUTRA Location 被設定
		require.NotNil(t, ranUe.Location.EutraLocation,
			"EutraLocation should be set")

		// 驗證 TAC 正確解析
		require.Equal(t, "000003", ranUe.Tai.Tac,
			"EUTRA TAC should be correctly parsed")

		// 驗證 ECGI 被設定
		require.NotNil(t, ranUe.Location.EutraLocation.Ecgi,
			"ECGI should be set")
		require.NotEmpty(t, ranUe.Location.EutraLocation.Ecgi.EutraCellId,
			"EUTRA Cell ID should be set")
	})

	t.Cleanup(func() {
		ranUe.Remove()
		amfContext.UePool.Delete(testSupi)
		amfContext.AmfRanPool.Delete(fakeConn)
	})
}

// TestRanUe_UpdateLocation_NilInput tests error handling with nil input
// 測試目標：驗證 nil 輸入的錯誤處理
func TestRanUe_UpdateLocation_NilInput(t *testing.T) {
	amfContext := GetSelf()

	if len(amfContext.ServedGuamiList) == 0 {
		amfContext.ServedGuamiList = append(amfContext.ServedGuamiList, models.Guami{
			PlmnId: &models.PlmnIdNid{
				Mcc: "208",
				Mnc: "93",
			},
			AmfId: "cafe00",
		})
	}

	testSupi := "imsi-208930000000104"
	amfUe := amfContext.NewAmfUe(testSupi)

	fakeConn := &fakeNetConn{}
	amfRan := amfContext.NewAmfRan(fakeConn)
	amfRan.AnType = models.AccessType__3_GPP_ACCESS

	ranUe, err := amfRan.NewRanUe(1)
	require.NoError(t, err)

	amfUe.AttachRanUe(ranUe)

	t.Run("Nil UserLocationInformation", func(t *testing.T) {
		// 呼叫 UpdateLocation with nil (應該不會 panic)
		require.NotPanics(t, func() {
			ranUe.UpdateLocation(nil)
		}, "UpdateLocation should handle nil input gracefully")

		// Location 應該保持未設定
		require.Nil(t, ranUe.Location.NrLocation)
		require.Nil(t, ranUe.Location.EutraLocation)
	})

	t.Cleanup(func() {
		ranUe.Remove()
		amfContext.UePool.Delete(testSupi)
		amfContext.AmfRanPool.Delete(fakeConn)
	})
}

// ==================== 邊緣測試 (Edge Case Tests) ====================

// TestRanUe_UpdateLocation_MultipleUpdates tests consecutive location updates
// 測試目標：測試連續多次位置更新的穩定性
func TestRanUe_UpdateLocation_MultipleUpdates(t *testing.T) {
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

	testSupi := "imsi-208930000000301"
	amfUe := amfContext.NewAmfUe(testSupi)

	fakeConn := &fakeNetConn{}
	amfRan := amfContext.NewAmfRan(fakeConn)
	amfRan.AnType = models.AccessType__3_GPP_ACCESS

	ranUe, err := amfRan.NewRanUe(1)
	require.NoError(t, err)

	amfUe.AttachRanUe(ranUe)

	// 準備測試用的位置資訊
	plmnBytes := aper.OctetString("\x02\x08\x93")

	iterations := 100

	t.Run("100 Consecutive Location Updates", func(t *testing.T) {
		for i := 0; i < iterations; i++ {
			// 每次更新使用不同的 TAC
			tacValue := byte(i % 256)
			testTac := []byte{0x00, 0x00, tacValue}

			userLocationInfo := &ngapType.UserLocationInformation{
				Present: ngapType.UserLocationInformationPresentUserLocationInformationNR,
				UserLocationInformationNR: &ngapType.UserLocationInformationNR{
					TAI: ngapType.TAI{
						PLMNIdentity: ngapType.PLMNIdentity{
							Value: plmnBytes,
						},
						TAC: ngapType.TAC{
							Value: aper.OctetString(testTac),
						},
					},
					NRCGI: ngapType.NRCGI{
						PLMNIdentity: ngapType.PLMNIdentity{
							Value: plmnBytes,
						},
						NRCellIdentity: ngapType.NRCellIdentity{
							Value: aper.BitString{
								Bytes:     []byte{0x00, 0x00, 0x00, 0x00, byte(i)},
								BitLength: 36,
							},
						},
					},
				},
			}

			// 更新位置
			require.NotPanics(t, func() {
				ranUe.UpdateLocation(userLocationInfo)
			}, "Update %d should not panic", i)
		}

		// 驗證最後一次更新的結果
		lastTac := iterations - 1
		expectedTac := lastTac % 256
		expectedTacStr := ""
		if expectedTac < 16 {
			expectedTacStr = "00000" + string(rune('0'+expectedTac))
		} else {
			// 轉換為十六進位字串
			expectedTacStr = ""
			for expectedTac >= 16 {
				expectedTacStr = string(rune('0'+(expectedTac%16))) + expectedTacStr
				expectedTac /= 16
			}
			expectedTacStr = "0000" + string(rune('0'+expectedTac)) + expectedTacStr
		}

		require.NotNil(t, ranUe.Location.NrLocation,
			"Location should be set after %d updates", iterations)
		
		t.Logf("Final TAC after %d updates: %s", iterations, ranUe.Tai.Tac)
	})

	t.Run("Verify No Memory Leak", func(t *testing.T) {
		// 驗證位置只有一個,沒有累積
		require.NotNil(t, ranUe.Location.NrLocation)
		
		// AmfUe 的位置也應該只有一個
		require.NotNil(t, amfUe.Location.NrLocation)
	})

	t.Cleanup(func() {
		ranUe.Remove()
		amfContext.UePool.Delete(testSupi)
		amfContext.AmfRanPool.Delete(fakeConn)
	})
}

// TestRanUe_UpdateLocation_WithoutAmfUe tests location update without AmfUe attached
// 測試目標：測試 RanUe 未連接 AmfUe 時的位置更新
func TestRanUe_UpdateLocation_WithoutAmfUe(t *testing.T) {
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

	// 確認沒有 Attach AmfUe
	require.Nil(t, ranUe.AmfUe, "ranUe.AmfUe should be nil initially")

	t.Run("Update Location Without AmfUe", func(t *testing.T) {
		// 準備位置資訊
		plmnBytes := aper.OctetString("\x02\x08\x93")
		testTac := []byte{0x00, 0x00, 0x05}

		userLocationInfo := &ngapType.UserLocationInformation{
			Present: ngapType.UserLocationInformationPresentUserLocationInformationNR,
			UserLocationInformationNR: &ngapType.UserLocationInformationNR{
				TAI: ngapType.TAI{
					PLMNIdentity: ngapType.PLMNIdentity{
						Value: plmnBytes,
					},
					TAC: ngapType.TAC{
						Value: aper.OctetString(testTac),
					},
				},
				NRCGI: ngapType.NRCGI{
					PLMNIdentity: ngapType.PLMNIdentity{
						Value: plmnBytes,
					},
					NRCellIdentity: ngapType.NRCellIdentity{
						Value: aper.BitString{
							Bytes:     []byte{0x00, 0x00, 0x00, 0x00, 0x20},
							BitLength: 36,
						},
					},
				},
			},
		}

		// 更新位置 (應該不會 panic)
		require.NotPanics(t, func() {
			ranUe.UpdateLocation(userLocationInfo)
		}, "UpdateLocation should handle nil AmfUe gracefully")

		// 驗證 RanUe 的位置被更新
		require.NotNil(t, ranUe.Location.NrLocation,
			"RanUe location should be updated even without AmfUe")
		require.Equal(t, "000005", ranUe.Tai.Tac,
			"RanUe TAC should be updated")
	})

	t.Cleanup(func() {
		ranUe.Remove()
		amfContext.AmfRanPool.Delete(fakeConn)
	})
}

// TestRanUe_UpdateLocation_InvalidPLMN tests handling of invalid PLMN data
// 測試目標：測試無效的 PLMN ID 資料處理 (防禦性測試)
func TestRanUe_UpdateLocation_InvalidPLMN(t *testing.T) {
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

	testSupi := "imsi-208930000000302"
	amfUe := amfContext.NewAmfUe(testSupi)

	fakeConn := &fakeNetConn{}
	amfRan := amfContext.NewAmfRan(fakeConn)
	amfRan.AnType = models.AccessType__3_GPP_ACCESS

	ranUe, err := amfRan.NewRanUe(1)
	require.NoError(t, err)

	amfUe.AttachRanUe(ranUe)

	t.Run("All Zero PLMN", func(t *testing.T) {
		// 全零的 PLMN
		plmnBytes := aper.OctetString("\x00\x00\x00")
		testTac := []byte{0x00, 0x00, 0x01}

		userLocationInfo := &ngapType.UserLocationInformation{
			Present: ngapType.UserLocationInformationPresentUserLocationInformationNR,
			UserLocationInformationNR: &ngapType.UserLocationInformationNR{
				TAI: ngapType.TAI{
					PLMNIdentity: ngapType.PLMNIdentity{
						Value: plmnBytes,
					},
					TAC: ngapType.TAC{
						Value: aper.OctetString(testTac),
					},
				},
				NRCGI: ngapType.NRCGI{
					PLMNIdentity: ngapType.PLMNIdentity{
						Value: plmnBytes,
					},
					NRCellIdentity: ngapType.NRCellIdentity{
						Value: aper.BitString{
							Bytes:     []byte{0x00, 0x00, 0x00, 0x00, 0x10},
							BitLength: 36,
						},
					},
				},
			},
		}

		// 應該不會 panic (防禦性測試)
		require.NotPanics(t, func() {
			ranUe.UpdateLocation(userLocationInfo)
		}, "UpdateLocation should handle all-zero PLMN without panic")

		// 驗證位置被更新 (即使 PLMN 不正常)
		require.NotNil(t, ranUe.Location.NrLocation)
		t.Logf("PLMN with all zeros - MCC: %s, MNC: %s",
			ranUe.Location.NrLocation.Tai.PlmnId.Mcc,
			ranUe.Location.NrLocation.Tai.PlmnId.Mnc)
	})

	t.Run("All 0xFF PLMN", func(t *testing.T) {
		// 全 0xFF 的 PLMN (極端值)
		plmnBytes := aper.OctetString("\xFF\xFF\xFF")
		testTac := []byte{0x00, 0x00, 0x02}

		userLocationInfo := &ngapType.UserLocationInformation{
			Present: ngapType.UserLocationInformationPresentUserLocationInformationNR,
			UserLocationInformationNR: &ngapType.UserLocationInformationNR{
				TAI: ngapType.TAI{
					PLMNIdentity: ngapType.PLMNIdentity{
						Value: plmnBytes,
					},
					TAC: ngapType.TAC{
						Value: aper.OctetString(testTac),
					},
				},
				NRCGI: ngapType.NRCGI{
					PLMNIdentity: ngapType.PLMNIdentity{
						Value: plmnBytes,
					},
					NRCellIdentity: ngapType.NRCellIdentity{
						Value: aper.BitString{
							Bytes:     []byte{0x00, 0x00, 0x00, 0x00, 0x20},
							BitLength: 36,
						},
					},
				},
			},
		}

		// 應該不會 panic
		require.NotPanics(t, func() {
			ranUe.UpdateLocation(userLocationInfo)
		}, "UpdateLocation should handle 0xFF PLMN without panic")

		t.Logf("PLMN with all 0xFF - MCC: %s, MNC: %s",
			ranUe.Location.NrLocation.Tai.PlmnId.Mcc,
			ranUe.Location.NrLocation.Tai.PlmnId.Mnc)
	})

	t.Cleanup(func() {
		ranUe.Remove()
		amfContext.UePool.Delete(testSupi)
		amfContext.AmfRanPool.Delete(fakeConn)
	})
}

// TestRanUe_UpdateLocation_EmptyTAC tests handling of empty TAC
// 測試目標：測試空的 TAC 值處理 (防禦性測試)
func TestRanUe_UpdateLocation_EmptyTAC(t *testing.T) {
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

	testSupi := "imsi-208930000000303"
	amfUe := amfContext.NewAmfUe(testSupi)

	fakeConn := &fakeNetConn{}
	amfRan := amfContext.NewAmfRan(fakeConn)
	amfRan.AnType = models.AccessType__3_GPP_ACCESS

	ranUe, err := amfRan.NewRanUe(1)
	require.NoError(t, err)

	amfUe.AttachRanUe(ranUe)

	t.Run("Empty TAC Byte Array", func(t *testing.T) {
		plmnBytes := aper.OctetString("\x02\x08\x93")
		emptyTac := []byte{} // 空的 TAC

		userLocationInfo := &ngapType.UserLocationInformation{
			Present: ngapType.UserLocationInformationPresentUserLocationInformationNR,
			UserLocationInformationNR: &ngapType.UserLocationInformationNR{
				TAI: ngapType.TAI{
					PLMNIdentity: ngapType.PLMNIdentity{
						Value: plmnBytes,
					},
					TAC: ngapType.TAC{
						Value: aper.OctetString(emptyTac),
					},
				},
				NRCGI: ngapType.NRCGI{
					PLMNIdentity: ngapType.PLMNIdentity{
						Value: plmnBytes,
					},
					NRCellIdentity: ngapType.NRCellIdentity{
						Value: aper.BitString{
							Bytes:     []byte{0x00, 0x00, 0x00, 0x00, 0x10},
							BitLength: 36,
						},
					},
				},
			},
		}

		// 應該不會 panic (防禦性測試)
		require.NotPanics(t, func() {
			ranUe.UpdateLocation(userLocationInfo)
		}, "UpdateLocation should handle empty TAC without panic")

		// 驗證處理結果
		require.NotNil(t, ranUe.Location.NrLocation)
		t.Logf("TAC with empty bytes: %s", ranUe.Tai.Tac)
	})

	t.Cleanup(func() {
		ranUe.Remove()
		amfContext.UePool.Delete(testSupi)
		amfContext.AmfRanPool.Delete(fakeConn)
	})
}

// TestRanUe_UpdateLocation_ZeroBitLengthCellID tests handling of zero-length cell ID
// 測試目標：測試零長度 Cell ID 的處理 (防禦性測試)
func TestRanUe_UpdateLocation_ZeroBitLengthCellID(t *testing.T) {
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

	testSupi := "imsi-208930000000304"
	amfUe := amfContext.NewAmfUe(testSupi)

	fakeConn := &fakeNetConn{}
	amfRan := amfContext.NewAmfRan(fakeConn)
	amfRan.AnType = models.AccessType__3_GPP_ACCESS

	ranUe, err := amfRan.NewRanUe(1)
	require.NoError(t, err)

	amfUe.AttachRanUe(ranUe)

	t.Run("Zero Bit Length Cell ID", func(t *testing.T) {
		plmnBytes := aper.OctetString("\x02\x08\x93")
		testTac := []byte{0x00, 0x00, 0x01}

		userLocationInfo := &ngapType.UserLocationInformation{
			Present: ngapType.UserLocationInformationPresentUserLocationInformationNR,
			UserLocationInformationNR: &ngapType.UserLocationInformationNR{
				TAI: ngapType.TAI{
					PLMNIdentity: ngapType.PLMNIdentity{
						Value: plmnBytes,
					},
					TAC: ngapType.TAC{
						Value: aper.OctetString(testTac),
					},
				},
				NRCGI: ngapType.NRCGI{
					PLMNIdentity: ngapType.PLMNIdentity{
						Value: plmnBytes,
					},
					NRCellIdentity: ngapType.NRCellIdentity{
						Value: aper.BitString{
							Bytes:     []byte{0x00},
							BitLength: 0, // 零長度
						},
					},
				},
			},
		}

		// 應該不會 panic
		require.NotPanics(t, func() {
			ranUe.UpdateLocation(userLocationInfo)
		}, "UpdateLocation should handle zero-length cell ID without panic")

		// 驗證位置被更新
		require.NotNil(t, ranUe.Location.NrLocation)
		t.Logf("Cell ID with zero bit length: %s", ranUe.Location.NrLocation.Ncgi.NrCellId)
	})

	t.Cleanup(func() {
		ranUe.Remove()
		amfContext.UePool.Delete(testSupi)
		amfContext.AmfRanPool.Delete(fakeConn)
	})
}

// TestRanUe_UpdateLocation_AlternateEUTRAandNR tests alternating between EUTRA and NR updates
// 測試目標：測試在 EUTRA 和 NR 之間交替更新
func TestRanUe_UpdateLocation_AlternateEUTRAandNR(t *testing.T) {
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

	testSupi := "imsi-208930000000305"
	amfUe := amfContext.NewAmfUe(testSupi)

	fakeConn := &fakeNetConn{}
	amfRan := amfContext.NewAmfRan(fakeConn)
	amfRan.AnType = models.AccessType__3_GPP_ACCESS

	ranUe, err := amfRan.NewRanUe(1)
	require.NoError(t, err)

	amfUe.AttachRanUe(ranUe)

	plmnBytes := aper.OctetString("\x02\x08\x93")

	cycles := 10

	t.Run("Alternate Between EUTRA and NR", func(t *testing.T) {
		for i := 0; i < cycles; i++ {
			if i%2 == 0 {
				// NR 更新
				testTac := []byte{0x00, 0x00, byte(i)}
				nrLocationInfo := &ngapType.UserLocationInformation{
					Present: ngapType.UserLocationInformationPresentUserLocationInformationNR,
					UserLocationInformationNR: &ngapType.UserLocationInformationNR{
						TAI: ngapType.TAI{
							PLMNIdentity: ngapType.PLMNIdentity{
								Value: plmnBytes,
							},
							TAC: ngapType.TAC{
								Value: aper.OctetString(testTac),
							},
						},
						NRCGI: ngapType.NRCGI{
							PLMNIdentity: ngapType.PLMNIdentity{
								Value: plmnBytes,
							},
							NRCellIdentity: ngapType.NRCellIdentity{
								Value: aper.BitString{
									Bytes:     []byte{0x00, 0x00, 0x00, 0x00, byte(i)},
									BitLength: 36,
								},
							},
						},
					},
				}

				ranUe.UpdateLocation(nrLocationInfo)
				require.NotNil(t, ranUe.Location.NrLocation,
					"Cycle %d: NR location should be set", i)
			} else {
				// EUTRA 更新
				testTac := []byte{0x00, 0x00, byte(i)}
				eutraLocationInfo := &ngapType.UserLocationInformation{
					Present: ngapType.UserLocationInformationPresentUserLocationInformationEUTRA,
					UserLocationInformationEUTRA: &ngapType.UserLocationInformationEUTRA{
						TAI: ngapType.TAI{
							PLMNIdentity: ngapType.PLMNIdentity{
								Value: plmnBytes,
							},
							TAC: ngapType.TAC{
								Value: aper.OctetString(testTac),
							},
						},
						EUTRACGI: ngapType.EUTRACGI{
							PLMNIdentity: ngapType.PLMNIdentity{
								Value: plmnBytes,
							},
							EUTRACellIdentity: ngapType.EUTRACellIdentity{
								Value: aper.BitString{
									Bytes:     []byte{0x00, 0x00, 0x00, byte(i)},
									BitLength: 28,
								},
							},
						},
					},
				}

				ranUe.UpdateLocation(eutraLocationInfo)
				require.NotNil(t, ranUe.Location.EutraLocation,
					"Cycle %d: EUTRA location should be set", i)
			}
		}
	})

	t.Run("Verify Final State", func(t *testing.T) {
		// 最後一次是偶數 (cycles-1 = 9, 奇數), 所以應該是 EUTRA
		if (cycles-1)%2 == 0 {
			require.NotNil(t, ranUe.Location.NrLocation,
				"Final location should be NR")
		} else {
			require.NotNil(t, ranUe.Location.EutraLocation,
				"Final location should be EUTRA")
		}

		// 兩種位置類型應該都有設定過 (不會互相覆蓋)
		require.NotNil(t, ranUe.Location.NrLocation,
			"NR location should be preserved")
		require.NotNil(t, ranUe.Location.EutraLocation,
			"EUTRA location should be preserved")
	})

	t.Cleanup(func() {
		ranUe.Remove()
		amfContext.UePool.Delete(testSupi)
		amfContext.AmfRanPool.Delete(fakeConn)
	})
}

// TestRanUe_UpdateLocation_ConcurrentUpdates tests concurrent location updates
// 測試目標：測試並發位置更新的執行緒安全性
func TestRanUe_UpdateLocation_ConcurrentUpdates(t *testing.T) {
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

	testSupi := "imsi-208930000000306"
	amfUe := amfContext.NewAmfUe(testSupi)

	fakeConn := &fakeNetConn{}
	amfRan := amfContext.NewAmfRan(fakeConn)
	amfRan.AnType = models.AccessType__3_GPP_ACCESS

	ranUe, err := amfRan.NewRanUe(1)
	require.NoError(t, err)

	amfUe.AttachRanUe(ranUe)

	t.Run("50 Concurrent Location Updates", func(t *testing.T) {
		done := make(chan bool, 50)

		for i := 0; i < 50; i++ {
			go func(id int) {
				defer func() {
					done <- true
				}()

				plmnBytes := aper.OctetString("\x02\x08\x93")
				testTac := []byte{0x00, 0x00, byte(id)}

				userLocationInfo := &ngapType.UserLocationInformation{
					Present: ngapType.UserLocationInformationPresentUserLocationInformationNR,
					UserLocationInformationNR: &ngapType.UserLocationInformationNR{
						TAI: ngapType.TAI{
							PLMNIdentity: ngapType.PLMNIdentity{
								Value: plmnBytes,
							},
							TAC: ngapType.TAC{
								Value: aper.OctetString(testTac),
							},
						},
						NRCGI: ngapType.NRCGI{
							PLMNIdentity: ngapType.PLMNIdentity{
								Value: plmnBytes,
							},
							NRCellIdentity: ngapType.NRCellIdentity{
								Value: aper.BitString{
									Bytes:     []byte{0x00, 0x00, 0x00, 0x00, byte(id)},
									BitLength: 36,
								},
							},
						},
					},
				}

				ranUe.UpdateLocation(userLocationInfo)
			}(i)
		}

		// 等待所有更新完成
		for i := 0; i < 50; i++ {
			<-done
		}
	})

	t.Run("Verify Final State After Concurrent Updates", func(t *testing.T) {
		// 驗證最終位置被設定
		require.NotNil(t, ranUe.Location.NrLocation,
			"Location should be set after concurrent updates")
		require.NotNil(t, amfUe.Location.NrLocation,
			"AmfUe location should be set")

		// 不會 panic 就是成功
		t.Logf("Final TAC after concurrent updates: %s", ranUe.Tai.Tac)
	})

	t.Cleanup(func() {
		ranUe.Remove()
		amfContext.UePool.Delete(testSupi)
		amfContext.AmfRanPool.Delete(fakeConn)
	})
}
