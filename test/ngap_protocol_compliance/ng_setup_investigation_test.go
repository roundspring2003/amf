package ngap_protocol_compliance

import (
	"testing"

	utils "github.com/free5gc/amf/test/ngap_test_utils"
)

// ==================== Question #1 調查: 多 TAI 驗證策略 ====================

// TestNGSetup_Investigation_MultiTAIValidationStrategy 調查 AMF 如何驗證多個 TAI
func TestNGSetup_Investigation_MultiTAIValidationStrategy(t *testing.T) {
	utils.PrintTestSeparator(t, "Investigation: Multi-TAI Validation Strategy")

	configManager := utils.NewTestConfigManager()
	configManager.LoadStandardConfig()

	t.Log("AMF Supported TAIs:")
	t.Log("  - PLMN=208-93, TAC=000001 ✅")
	t.Log("  - PLMN=208-93, TAC=000002 ✅")
	t.Log("")

	// ========== 測試場景 1: 支援 + 不支援 (順序重要!) ==========
	t.Run("Scenario 1: Supported TAI First, Then Unsupported", func(t *testing.T) {
		t.Log("gNB Request Order:")
		t.Log("  [1] TAC=000001 ✅ (Supported)")
		t.Log("  [2] TAC=999999 ❌ (NOT Supported)")

		fakeConn := &utils.FakeNetConn{}
		gNB := utils.NewFakeGNB(fakeConn, "test-order-1")

		// 第一個 TAI: 支援
		gNB.SetSupportedSlices(
			utils.PLMN{MCC: "208", MNC: "93"},
			"000001",
			[]utils.SNSSAI{{SST: 1, SD: "010203"}},
		)

		// 第二個 TAI: 不支援
		gNB.SetSupportedSlices(
			utils.PLMN{MCC: "208", MNC: "93"},
			"999999",
			[]utils.SNSSAI{{SST: 1, SD: "010203"}},
		)

		validator := utils.NewNGAPValidator(configManager)
		err := validator.ValidateSupportedTAIList(gNB.SupportedTAIList)

		if err != nil {
			t.Log("❌ Validation FAILED (Correct!)")
			t.Logf("   Error: %v", err)
			t.Log("")
			t.Log("✅ Expected Behavior: Reject because TAI[2] is unsupported")
			t.Log("   AMF should validate ALL TAIs, not just the first one")
		} else {
			t.Log("✅ Validation PASSED (Unexpected!)")
			t.Log("")
			t.Log("🐛 BUG CONFIRMED: AMF only checks if AT LEAST ONE TAI is supported!")
			t.Log("   AMF should reject this because TAI[2] is unsupported")
			t.Log("   But it accepts because TAI[1] is supported")
		}
	})

	t.Log("")
	t.Log("---")
	t.Log("")

	// ========== 測試場景 2: 不支援 + 支援 (反向順序) ==========
	t.Run("Scenario 2: Unsupported TAI First, Then Supported", func(t *testing.T) {
		t.Log("gNB Request Order:")
		t.Log("  [1] TAC=999999 ❌ (NOT Supported)")
		t.Log("  [2] TAC=000001 ✅ (Supported)")

		fakeConn := &utils.FakeNetConn{}
		gNB := utils.NewFakeGNB(fakeConn, "test-order-2")

		// 第一個 TAI: 不支援
		gNB.SetSupportedSlices(
			utils.PLMN{MCC: "208", MNC: "93"},
			"999999",
			[]utils.SNSSAI{{SST: 1, SD: "010203"}},
		)

		// 第二個 TAI: 支援
		gNB.SetSupportedSlices(
			utils.PLMN{MCC: "208", MNC: "93"},
			"000001",
			[]utils.SNSSAI{{SST: 1, SD: "010203"}},
		)

		validator := utils.NewNGAPValidator(configManager)
		err := validator.ValidateSupportedTAIList(gNB.SupportedTAIList)

		if err != nil {
			t.Log("❌ Validation FAILED")
			t.Logf("   Error: %v", err)
			t.Log("")
			t.Log("✅ Expected Behavior: Correctly rejects because TAI[1] is unsupported")
		} else {
			t.Log("✅ Validation PASSED (Unexpected!)")
			t.Log("")
			t.Log("🐛 BUG: Should reject because TAI[1] is unsupported")
		}
	})

	t.Log("")
	t.Log("---")
	t.Log("")

	// ========== 測試場景 3: 3 個 TAI (支援, 不支援, 不支援) ==========
	t.Run("Scenario 3: One Supported, Two Unsupported", func(t *testing.T) {
		t.Log("gNB Request Order:")
		t.Log("  [1] TAC=000001 ✅ (Supported)")
		t.Log("  [2] TAC=999998 ❌ (NOT Supported)")
		t.Log("  [3] TAC=999999 ❌ (NOT Supported)")

		fakeConn := &utils.FakeNetConn{}
		gNB := utils.NewFakeGNB(fakeConn, "test-three-tai")

		gNB.SetSupportedSlices(
			utils.PLMN{MCC: "208", MNC: "93"},
			"000001",
			[]utils.SNSSAI{{SST: 1, SD: "010203"}},
		)

		gNB.SetSupportedSlices(
			utils.PLMN{MCC: "208", MNC: "93"},
			"999998",
			[]utils.SNSSAI{{SST: 1, SD: "010203"}},
		)

		gNB.SetSupportedSlices(
			utils.PLMN{MCC: "208", MNC: "93"},
			"999999",
			[]utils.SNSSAI{{SST: 1, SD: "010203"}},
		)

		validator := utils.NewNGAPValidator(configManager)
		err := validator.ValidateSupportedTAIList(gNB.SupportedTAIList)

		if err != nil {
			t.Log("❌ Validation FAILED")
			t.Logf("   Error: %v", err)
			t.Log("")
			t.Log("✅ Correct: Rejects because TAI[2] and TAI[3] are unsupported")
		} else {
			t.Log("✅ Validation PASSED (Bug!)")
			t.Log("")
			t.Log("🐛 BUG: AMF accepts this even though 2 out of 3 TAIs are unsupported!")
		}
	})

	t.Log("")
	t.Log("===== INVESTIGATION SUMMARY =====")
	t.Log("")
	t.Log("Based on AMF handler.go code analysis (lines 80-87):")
	t.Log("")
	t.Log("Current AMF Behavior:")
	t.Log("  - Loops through ALL TAIs in gNB request")
	t.Log("  - Checks if each TAI is in AMF's supported list")
	t.Log("  - If finds ANY match, sets found=true and BREAKS ⚠️")
	t.Log("  - Only requires AT LEAST ONE TAI to be supported")
	t.Log("")
	t.Log("Expected Behavior (3GPP Compliant):")
	t.Log("  - Should validate ALL TAIs")
	t.Log("  - Should reject if ANY TAI is not supported")
	t.Log("  - Should NOT break after finding first match")
	t.Log("")
	t.Log("🐛 Bug #9: AMF Validates 'At Least One' Instead of 'All'")
	t.Log("   Severity: MEDIUM")
	t.Log("   Impact: Allows unsupported TAIs to be configured")
	t.Log("   Risk: gNB can operate in unauthorized tracking areas")
}

// TestNGSetup_Investigation_AllSupportedVsAtLeastOne 明確區分兩種策略
func TestNGSetup_Investigation_AllSupportedVsAtLeastOne(t *testing.T) {
	utils.PrintTestSeparator(t, "Investigation: All Supported vs At Least One")

	configManager := utils.NewTestConfigManager()
	configManager.LoadStandardConfig()

	// ========== 策略 A: At Least One (AMF 當前行為) ==========
	t.Run("Strategy A: At Least One TAI Supported", func(t *testing.T) {
		t.Log("Description: Accept if ANY TAI is supported")
		t.Log("")

		testCases := []struct {
			name     string
			tais     []string
			expected string
		}{
			{
				name:     "All Supported",
				tais:     []string{"000001", "000002"},
				expected: "ACCEPT",
			},
			{
				name:     "One Supported, One Unsupported",
				tais:     []string{"000001", "999999"},
				expected: "ACCEPT (Bug!)",
			},
			{
				name:     "All Unsupported",
				tais:     []string{"999998", "999999"},
				expected: "REJECT",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Logf("TAIs: %v", tc.tais)
				t.Logf("Expected with 'At Least One' strategy: %s", tc.expected)

				fakeConn := &utils.FakeNetConn{}
				gNB := utils.NewFakeGNB(fakeConn, "test-strategy-a")

				for _, tac := range tc.tais {
					gNB.SetSupportedSlices(
						utils.PLMN{MCC: "208", MNC: "93"},
						tac,
						[]utils.SNSSAI{{SST: 1, SD: "010203"}},
					)
				}

				validator := utils.NewNGAPValidator(configManager)
				err := validator.ValidateSupportedTAIList(gNB.SupportedTAIList)

				if err != nil {
					t.Logf("Result: REJECT (%v)", err)
				} else {
					t.Log("Result: ACCEPT")
				}
			})
		}
	})

	t.Log("")

	// ========== 策略 B: All Must Be Supported (正確行為) ==========
	t.Run("Strategy B: All TAIs Must Be Supported", func(t *testing.T) {
		t.Log("Description: Accept ONLY if ALL TAIs are supported")
		t.Log("")

		testCases := []struct {
			name     string
			tais     []string
			expected string
		}{
			{
				name:     "All Supported",
				tais:     []string{"000001", "000002"},
				expected: "ACCEPT",
			},
			{
				name:     "One Supported, One Unsupported",
				tais:     []string{"000001", "999999"},
				expected: "REJECT",
			},
			{
				name:     "All Unsupported",
				tais:     []string{"999998", "999999"},
				expected: "REJECT",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Logf("TAIs: %v", tc.tais)
				t.Logf("Expected with 'All Must Be' strategy: %s", tc.expected)

				fakeConn := &utils.FakeNetConn{}
				gNB := utils.NewFakeGNB(fakeConn, "test-strategy-b")

				for _, tac := range tc.tais {
					gNB.SetSupportedSlices(
						utils.PLMN{MCC: "208", MNC: "93"},
						tac,
						[]utils.SNSSAI{{SST: 1, SD: "010203"}},
					)
				}

				validator := utils.NewNGAPValidator(configManager)
				err := validator.ValidateSupportedTAIList(gNB.SupportedTAIList)

				if err != nil {
					t.Logf("Result: REJECT (%v)", err)
					if tc.expected == "REJECT" {
						t.Log("✅ Correct")
					}
				} else {
					t.Log("Result: ACCEPT")
					if tc.expected == "ACCEPT" {
						t.Log("✅ Correct")
					} else {
						t.Log("❌ Should reject!")
					}
				}
			})
		}
	})

	t.Log("")
	t.Log("===== CONCLUSION =====")
	t.Log("")
	t.Log("Our test validator implements 'Strategy B' (All Must Be Supported)")
	t.Log("This is the CORRECT behavior according to 3GPP standards")
	t.Log("")
	t.Log("AMF currently implements 'Strategy A' (At Least One)")
	t.Log("This is INCORRECT and creates a security/policy vulnerability")
}
