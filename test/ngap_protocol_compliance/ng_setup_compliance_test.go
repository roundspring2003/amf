package ngap_protocol_compliance

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	utils "github.com/free5gc/amf/test/ngap_test_utils"
)

// ==================== 基礎測試 (已有的 5 個) ====================

// TestNGSetup_StandardConfiguration 測試標準配置下的 NG Setup
func TestNGSetup_StandardConfiguration(t *testing.T) {
	utils.PrintTestSeparator(t, "NG Setup - Standard Configuration")

	configManager := utils.NewTestConfigManager()
	config := configManager.LoadStandardConfig()

	t.Logf("AMF Configuration:")
	t.Logf("  - AMF Name: %s", config.AMFName)
	t.Logf("  - Supported PLMNs: %d", len(config.SupportedPLMNs))
	t.Logf("  - Supported TAIs: %d", len(config.SupportedTAIList))

	fakeConn := &utils.FakeNetConn{}
	gNB := utils.NewFakeGNB(fakeConn, "test-gnb-1")

	gNB.SetSupportedSlices(
		utils.PLMN{MCC: "208", MNC: "93"},
		"000001",
		[]utils.SNSSAI{
			{SST: 1, SD: "010203"},
		},
	)

	t.Logf("gNB Configuration:")
	t.Logf("  - Name: %s", gNB.RANNodeName)
	t.Logf("  - Supported TAIs: %d", len(gNB.SupportedTAIList))

	pdu, err := gNB.SendNGSetupRequest()
	require.NoError(t, err)
	require.NotNil(t, pdu)

	t.Log("✅ NGSetupRequest built successfully")

	validator := utils.NewNGAPValidator(configManager)
	err = validator.ValidateSupportedTAIList(gNB.SupportedTAIList)
	require.NoError(t, err)

	t.Log("✅ All S-NSSAIs are supported - NG Setup should succeed")
}

// TestNGSetup_UnsupportedSlice 測試不支援的切片 (Bug 重現)
func TestNGSetup_UnsupportedSlice(t *testing.T) {
	utils.PrintTestSeparator(t, "NG Setup - Unsupported Slice (Bug Case)")

	configManager := utils.NewTestConfigManager()
	config := configManager.LoadStandardConfig()

	t.Logf("AMF Supported Slices:")
	for key, slices := range config.SupportedSlices {
		t.Logf("  - %s:", key)
		for _, s := range slices {
			t.Logf("    * %s", utils.FormatSNSSAI(s))
		}
	}

	fakeConn := &utils.FakeNetConn{}
	gNB := utils.NewFakeGNB(fakeConn, "test-gnb-2")

	unsupportedSlice := utils.SNSSAI{SST: 1, SD: "FEDCBA"}
	gNB.SetSupportedSlices(
		utils.PLMN{MCC: "208", MNC: "93"},
		"000001",
		[]utils.SNSSAI{unsupportedSlice},
	)

	t.Logf("gNB Configuration:")
	t.Logf("  - Requested Slice: %s ❌ (NOT SUPPORTED)", utils.FormatSNSSAI(unsupportedSlice))

	pdu, err := gNB.SendNGSetupRequest()
	require.NoError(t, err)
	require.NotNil(t, pdu)

	validator := utils.NewNGAPValidator(configManager)
	err = validator.ValidateSupportedTAIList(gNB.SupportedTAIList)
	require.Error(t, err)
	t.Logf("❌ Validation failed as expected: %v", err)

	t.Log("✅ Bug reproduced: AMF should reject this NG Setup Request")
	t.Log("   Expected: NGSetupFailure with Cause 'No Network Slices Available'")
}

// TestNGSetup_MixedSlices 測試部分支援的切片
func TestNGSetup_MixedSlices(t *testing.T) {
	utils.PrintTestSeparator(t, "NG Setup - Mixed Supported/Unsupported Slices")

	configManager := utils.NewTestConfigManager()
	configManager.LoadStandardConfig()

	fakeConn := &utils.FakeNetConn{}
	gNB := utils.NewFakeGNB(fakeConn, "test-gnb-3")

	gNB.SetSupportedSlices(
		utils.PLMN{MCC: "208", MNC: "93"},
		"000001",
		[]utils.SNSSAI{
			{SST: 1, SD: "010203"},
			{SST: 1, SD: "FEDCBA"},
		},
	)

	t.Log("gNB Requested Slices:")
	t.Log("  - SST=1, SD=010203 ✅ (Supported)")
	t.Log("  - SST=1, SD=FEDCBA ❌ (NOT Supported)")

	validator := utils.NewNGAPValidator(configManager)
	err := validator.ValidateSupportedTAIList(gNB.SupportedTAIList)
	require.Error(t, err)
	t.Logf("❌ Validation failed: %v", err)

	t.Log("✅ Correct behavior: NG Setup should be rejected if ANY slice is unsupported")
}

// TestNGSetup_UnsupportedPLMN 測試不支援的 PLMN
func TestNGSetup_UnsupportedPLMN(t *testing.T) {
	utils.PrintTestSeparator(t, "NG Setup - Unsupported PLMN")

	configManager := utils.NewTestConfigManager()
	configManager.LoadStandardConfig()

	fakeConn := &utils.FakeNetConn{}
	gNB := utils.NewFakeGNB(fakeConn, "test-gnb-4")

	unsupportedPLMN := utils.PLMN{MCC: "999", MNC: "99"}
	gNB.SetSupportedSlices(
		unsupportedPLMN,
		"000001",
		[]utils.SNSSAI{
			{SST: 1, SD: "010203"},
		},
	)

	t.Logf("gNB Requested PLMN: %s ❌ (NOT SUPPORTED)", utils.FormatPLMN(unsupportedPLMN))

	validator := utils.NewNGAPValidator(configManager)
	err := validator.ValidateSupportedTAIList(gNB.SupportedTAIList)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported PLMN")
	t.Logf("❌ Validation failed as expected: %v", err)

	t.Log("✅ NG Setup should be rejected with Cause 'Unknown PLMN'")
}

// TestNGSetup_EmptyTAIList 測試空的 TAI 列表
func TestNGSetup_EmptyTAIList(t *testing.T) {
	utils.PrintTestSeparator(t, "NG Setup - Empty TAI List")

	configManager := utils.NewTestConfigManager()
	configManager.LoadStandardConfig()

	emptyTAIList := []utils.SupportedTAI{}
	t.Log("gNB Supported TAI List: [] (EMPTY)")

	validator := utils.NewNGAPValidator(configManager)
	err := validator.ValidateSupportedTAIList(emptyTAIList)
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty")
	t.Logf("❌ Validation failed as expected: %v", err)

	t.Log("✅ NG Setup should be rejected - Missing Mandatory IE")
}

// ==================== TAC 驗證測試 (新增 3 個) ====================

// TestNGSetup_UnsupportedTAC 測試不支援的 TAC
func TestNGSetup_UnsupportedTAC(t *testing.T) {
	utils.PrintTestSeparator(t, "NG Setup - Unsupported TAC")

	configManager := utils.NewTestConfigManager()
	configManager.LoadStandardConfig()

	t.Log("AMF Supported TACs:")
	t.Log("  - 000001 ✅")
	t.Log("  - 000002 ✅")

	fakeConn := &utils.FakeNetConn{}
	gNB := utils.NewFakeGNB(fakeConn, "test-gnb-tac")

	unsupportedTAC := "999999"
	gNB.SetSupportedSlices(
		utils.PLMN{MCC: "208", MNC: "93"},
		unsupportedTAC,
		[]utils.SNSSAI{
			{SST: 1, SD: "010203"},
		},
	)

	t.Logf("gNB Requested TAC: %s ❌ (NOT SUPPORTED)", unsupportedTAC)

	validator := utils.NewNGAPValidator(configManager)
	err := validator.ValidateSupportedTAIList(gNB.SupportedTAIList)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported TAC")
	t.Logf("❌ Validation failed as expected: %v", err)

	t.Log("✅ NG Setup should be rejected with Cause 'Unknown TAC'")
	t.Log("🐛 Potential Bug: AMF may not validate TAC properly")
}

// TestNGSetup_InvalidTACFormat 測試無效的 TAC 格式
func TestNGSetup_InvalidTACFormat(t *testing.T) {
	utils.PrintTestSeparator(t, "NG Setup - Invalid TAC Format")

	configManager := utils.NewTestConfigManager()
	configManager.LoadStandardConfig()

	testCases := []struct {
		name        string
		tac         string
		description string
	}{
		{"Too Short TAC", "01", "TAC 只有 1 byte (應該是 3 bytes)"},
		{"Too Long TAC", "00000001", "TAC 有 4 bytes (應該是 3 bytes)"},
		{"All Zero TAC", "000000", "全零 TAC (可能無效)"},
		{"All FF TAC", "FFFFFF", "全 0xFF TAC (可能保留值)"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing: %s", tc.description)
			t.Logf("TAC: %s", tc.tac)

			fakeConn := &utils.FakeNetConn{}
			gNB := utils.NewFakeGNB(fakeConn, "test-gnb-tac-format")

			gNB.SetSupportedSlices(
				utils.PLMN{MCC: "208", MNC: "93"},
				tc.tac,
				[]utils.SNSSAI{{SST: 1, SD: "010203"}},
			)

			pdu, err := gNB.SendNGSetupRequest()
			require.NoError(t, err)
			require.NotNil(t, pdu)

			t.Log("✅ Message built without crash")
			t.Log("🐛 Potential Bug: AMF may not validate TAC format")
		})
	}
}

// TestNGSetup_MultipleTAIsWithPartialSupport 測試部分支援的 TAI 列表
func TestNGSetup_MultipleTAIsWithPartialSupport(t *testing.T) {
	utils.PrintTestSeparator(t, "NG Setup - Multiple TAIs with Partial Support")

	configManager := utils.NewTestConfigManager()
	configManager.LoadStandardConfig()

	fakeConn := &utils.FakeNetConn{}
	gNB := utils.NewFakeGNB(fakeConn, "test-gnb-multi-tai")

	gNB.SetSupportedSlices(
		utils.PLMN{MCC: "208", MNC: "93"},
		"000001",
		[]utils.SNSSAI{{SST: 1, SD: "010203"}},
	)

	gNB.SetSupportedSlices(
		utils.PLMN{MCC: "208", MNC: "93"},
		"999999",
		[]utils.SNSSAI{{SST: 1, SD: "010203"}},
	)

	t.Log("gNB Requested TAIs:")
	t.Log("  - TAI 1: PLMN=208-93, TAC=000001 ✅ (Supported)")
	t.Log("  - TAI 2: PLMN=208-93, TAC=999999 ❌ (NOT Supported)")

	validator := utils.NewNGAPValidator(configManager)
	err := validator.ValidateSupportedTAIList(gNB.SupportedTAIList)
	require.Error(t, err)
	t.Logf("❌ Validation failed: %v", err)

	t.Log("✅ Correct behavior: NG Setup should be rejected if ANY TAI is unsupported")
	t.Log("🐛 Question: Does AMF validate ALL TAIs or just the first one?")
}

// ==================== 配置邊界測試 (新增 4 個) ====================

// TestNGSetup_MaximumNumberOfSlices 測試最大切片數量
func TestNGSetup_MaximumNumberOfSlices(t *testing.T) {
	utils.PrintTestSeparator(t, "NG Setup - Maximum Number of Slices")

	configManager := utils.NewTestConfigManager()
	config := configManager.LoadStandardConfig()

	fakeConn := &utils.FakeNetConn{}
	gNB := utils.NewFakeGNB(fakeConn, "test-gnb-max-slices")

	const maxSlices = 8
	slices := make([]utils.SNSSAI, maxSlices)
	for i := 0; i < maxSlices; i++ {
		slices[i] = utils.SNSSAI{SST: 1, SD: fmt.Sprintf("%06x", i+1)}
		config.SupportedSlices["208-93-000001"] = append(
			config.SupportedSlices["208-93-000001"],
			slices[i],
		)
	}

	gNB.SetSupportedSlices(
		utils.PLMN{MCC: "208", MNC: "93"},
		"000001",
		slices,
	)

	t.Logf("gNB Requested Slices: %d (at maximum limit)", maxSlices)

	pdu, err := gNB.SendNGSetupRequest()
	require.NoError(t, err)
	require.NotNil(t, pdu)

	validator := utils.NewNGAPValidator(configManager)
	err = validator.ValidateSupportedTAIList(gNB.SupportedTAIList)
	require.NoError(t, err)

	t.Log("✅ Maximum slices accepted")
}

// TestNGSetup_ExceedMaximumSlices 測試超過最大切片數量
func TestNGSetup_ExceedMaximumSlices(t *testing.T) {
	utils.PrintTestSeparator(t, "NG Setup - Exceed Maximum Slices")

	configManager := utils.NewTestConfigManager()
	config := configManager.LoadStandardConfig()

	fakeConn := &utils.FakeNetConn{}
	gNB := utils.NewFakeGNB(fakeConn, "test-gnb-exceed-slices")

	const excessSlices = 16
	slices := make([]utils.SNSSAI, excessSlices)
	for i := 0; i < excessSlices; i++ {
		slices[i] = utils.SNSSAI{SST: 1, SD: fmt.Sprintf("%06x", i+1)}
		config.SupportedSlices["208-93-000001"] = append(
			config.SupportedSlices["208-93-000001"],
			slices[i],
		)
	}

	gNB.SetSupportedSlices(
		utils.PLMN{MCC: "208", MNC: "93"},
		"000001",
		slices,
	)

	t.Logf("gNB Requested Slices: %d ❌ (EXCEEDS maximum of 8)", excessSlices)

	pdu, err := gNB.SendNGSetupRequest()
	require.NoError(t, err)
	require.NotNil(t, pdu)

	t.Log("✅ Message built without crash")
	t.Log("🐛 Potential Bug: AMF may not validate maximum slice limit")
	t.Log("   Expected: NGSetupFailure or truncate to 8 slices")
}

// TestNGSetup_DuplicateSlicesInSameTAI 測試同一 TAI 中的重複切片
func TestNGSetup_DuplicateSlicesInSameTAI(t *testing.T) {
	utils.PrintTestSeparator(t, "NG Setup - Duplicate Slices in Same TAI")

	configManager := utils.NewTestConfigManager()
	configManager.LoadStandardConfig()

	fakeConn := &utils.FakeNetConn{}
	gNB := utils.NewFakeGNB(fakeConn, "test-gnb-dup-slices")

	duplicateSlice := utils.SNSSAI{SST: 1, SD: "010203"}
	gNB.SetSupportedSlices(
		utils.PLMN{MCC: "208", MNC: "93"},
		"000001",
		[]utils.SNSSAI{duplicateSlice, duplicateSlice, duplicateSlice},
	)

	t.Log("gNB Requested Slices:")
	t.Log("  - SST=1, SD=010203")
	t.Log("  - SST=1, SD=010203 (DUPLICATE)")
	t.Log("  - SST=1, SD=010203 (DUPLICATE)")

	pdu, err := gNB.SendNGSetupRequest()
	require.NoError(t, err)
	require.NotNil(t, pdu)

	t.Log("✅ Message built without crash")
	t.Log("🐛 Potential Bug: AMF may not detect duplicate slices")
	t.Log("   Expected: NGSetupFailure or deduplicate automatically")
}

// TestNGSetup_VeryLongRANNodeName 測試超長的 RAN Node Name
func TestNGSetup_VeryLongRANNodeName(t *testing.T) {
	utils.PrintTestSeparator(t, "NG Setup - Very Long RAN Node Name")

	configManager := utils.NewTestConfigManager()
	configManager.LoadStandardConfig()

	fakeConn := &utils.FakeNetConn{}
	gNB := utils.NewFakeGNB(fakeConn, "test-gnb-long-name")

	longName := strings.Repeat("A", 150)
	gNB.RANNodeName = longName

	gNB.SetSupportedSlices(
		utils.PLMN{MCC: "208", MNC: "93"},
		"000001",
		[]utils.SNSSAI{{SST: 1, SD: "010203"}},
	)

	t.Logf("RAN Node Name Length: %d characters (at limit)", len(longName))

	pdu, err := gNB.SendNGSetupRequest()
	require.NoError(t, err)
	require.NotNil(t, pdu)

	t.Log("✅ Message built successfully with 150-char name")

	t.Run("Exceed Name Length Limit", func(t *testing.T) {
		veryLongName := strings.Repeat("B", 200)
		gNB.RANNodeName = veryLongName

		t.Logf("RAN Node Name Length: %d characters ❌ (EXCEEDS limit)", len(veryLongName))

		pdu, err := gNB.SendNGSetupRequest()
		require.NoError(t, err)
		require.NotNil(t, pdu)

		t.Log("✅ Message built without crash")
		t.Log("🐛 Potential Bug: AMF may not validate name length")
		t.Log("   Expected: NGSetupFailure or truncate to 150 chars")
	})
}

// ==================== 訊息格式測試 (新增 3 個) ====================

// TestNGSetup_InvalidGlobalRANNodeID 測試無效的 Global RAN Node ID
func TestNGSetup_InvalidGlobalRANNodeID(t *testing.T) {
	utils.PrintTestSeparator(t, "NG Setup - Invalid Global RAN Node ID")

	builder := utils.NewNGAPMessageBuilder()
	
	t.Run("Missing Global RAN Node ID", func(t *testing.T) {
		pdu := builder.BuildInvalidNGSetupRequest("missing_mandatory_ie")
		require.NotNil(t, pdu)

		t.Log("Built NGSetupRequest without Global RAN Node ID")
		t.Log("🐛 Potential Bug: AMF may not detect missing mandatory IE")
		t.Log("   Expected: NGSetupFailure with Cause 'Missing Mandatory IE'")
	})
}

// TestNGSetup_MalformedPLMNID 測試畸形的 PLMN ID
func TestNGSetup_MalformedPLMNID(t *testing.T) {
	utils.PrintTestSeparator(t, "NG Setup - Malformed PLMN ID")

	configManager := utils.NewTestConfigManager()
	configManager.LoadStandardConfig()

	testCases := []struct {
		name string
		plmn utils.PLMN
		desc string
	}{
		{"Invalid MCC Length", utils.PLMN{MCC: "20", MNC: "93"}, "MCC 只有 2 位數"},
		{"Invalid MNC Length", utils.PLMN{MCC: "208", MNC: "9"}, "MNC 只有 1 位數"},
		{"Non-numeric MCC", utils.PLMN{MCC: "ABC", MNC: "93"}, "MCC 包含非數字字符"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing: %s", tc.desc)

			fakeConn := &utils.FakeNetConn{}
			gNB := utils.NewFakeGNB(fakeConn, "test-gnb-malformed-plmn")

			gNB.SetSupportedSlices(tc.plmn, "000001", []utils.SNSSAI{{SST: 1, SD: "010203"}})

			pdu, err := gNB.SendNGSetupRequest()
			require.NoError(t, err)
			require.NotNil(t, pdu)

			t.Log("✅ Message built without crash")
			t.Log("🐛 Potential Bug: AMF may not validate PLMN format")
		})
	}
}

// TestNGSetup_InvalidSliceConfiguration 測試無效的切片配置
func TestNGSetup_InvalidSliceConfiguration(t *testing.T) {
	utils.PrintTestSeparator(t, "NG Setup - Invalid Slice Configuration")

	configManager := utils.NewTestConfigManager()
	configManager.LoadStandardConfig()

	testCases := []struct {
		name  string
		slice utils.SNSSAI
		desc  string
	}{
		{"Invalid SST Value", utils.SNSSAI{SST: 0, SD: "010203"}, "SST = 0 (可能無效)"},
		{"Invalid SST 256", utils.SNSSAI{SST: 256, SD: "010203"}, "SST = 256 (超出範圍)"},
		{"Empty SD", utils.SNSSAI{SST: 1, SD: ""}, "SD 為空字串"},
		{"Invalid SD Length", utils.SNSSAI{SST: 1, SD: "01"}, "SD 只有 1 byte"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing: %s", tc.desc)

			fakeConn := &utils.FakeNetConn{}
			gNB := utils.NewFakeGNB(fakeConn, "test-gnb-invalid-slice")

			gNB.SetSupportedSlices(
				utils.PLMN{MCC: "208", MNC: "93"},
				"000001",
				[]utils.SNSSAI{tc.slice},
			)

			pdu, err := gNB.SendNGSetupRequest()
			require.NoError(t, err)
			require.NotNil(t, pdu)

			t.Log("✅ Message built without crash")
			t.Log("🐛 Potential Bug: AMF may not validate slice parameters")
		})
	}
}

// ==================== 安全性測試 (新增 2 個) ====================

// TestNGSetup_RapidRepeatedRequests 測試快速重複的 NG Setup 請求
func TestNGSetup_RapidRepeatedRequests(t *testing.T) {
	utils.PrintTestSeparator(t, "NG Setup - Rapid Repeated Requests")

	configManager := utils.NewTestConfigManager()
	configManager.LoadStandardConfig()

	fakeConn := &utils.FakeNetConn{}
	gNB := utils.NewFakeGNB(fakeConn, "test-gnb-rapid")

	gNB.SetSupportedSlices(
		utils.PLMN{MCC: "208", MNC: "93"},
		"000001",
		[]utils.SNSSAI{{SST: 1, SD: "010203"}},
	)

	const numRequests = 100
	t.Logf("Sending %d rapid NG Setup Requests...", numRequests)

	for i := 0; i < numRequests; i++ {
		pdu, err := gNB.SendNGSetupRequest()
		require.NoError(t, err)
		require.NotNil(t, pdu)
	}

	t.Logf("✅ Successfully sent %d requests without crash", numRequests)
	t.Log("🐛 Question: Does AMF handle rapid repeated NG Setup properly?")
}

// TestNGSetup_ConcurrentFromMultipleGNBs 測試多個 gNB 同時發送 NG Setup
func TestNGSetup_ConcurrentFromMultipleGNBs(t *testing.T) {
	utils.PrintTestSeparator(t, "NG Setup - Concurrent from Multiple gNBs")

	configManager := utils.NewTestConfigManager()
	configManager.LoadStandardConfig()

	const numGNBs = 10
	t.Logf("Simulating %d gNBs sending NG Setup concurrently...", numGNBs)

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < numGNBs; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			fakeConn := &utils.FakeNetConn{}
			gNB := utils.NewFakeGNB(fakeConn, fmt.Sprintf("gnb-%d", id))

			gNB.SetSupportedSlices(
				utils.PLMN{MCC: "208", MNC: "93"},
				"000001",
				[]utils.SNSSAI{{SST: 1, SD: "010203"}},
			)

			pdu, err := gNB.SendNGSetupRequest()
			if err == nil && pdu != nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	t.Logf("✅ Successfully built %d/%d concurrent requests", successCount, numGNBs)
	require.Equal(t, numGNBs, successCount)
	t.Log("🐛 Question: Does AMF handle concurrent NG Setups from different gNBs?")
}
