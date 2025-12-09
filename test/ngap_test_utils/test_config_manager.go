package ngap_test_utils

import (
	"github.com/free5gc/openapi/models"
)

// TestConfigManager 管理測試用的 AMF 配置
type TestConfigManager struct {
	config *TestAMFConfig
}

// NewTestConfigManager 建立一個新的測試配置管理器
func NewTestConfigManager() *TestConfigManager {
	return &TestConfigManager{
		config: &TestAMFConfig{
			SupportedSlices: make(map[string][]SNSSAI),
		},
	}
}

// LoadStandardConfig 載入標準測試配置
// 支援: PLMN 208-93, TAC 000001, Slices: 1-010203, 1-112233
func (m *TestConfigManager) LoadStandardConfig() *TestAMFConfig {
	m.config = &TestAMFConfig{
		AMFName: "TestAMF",
		ServedGUAMIList: []models.Guami{
			{
				PlmnId: &models.PlmnIdNid{
					Mcc: "208",
					Mnc: "93",
				},
				AmfId: "cafe00",
			},
		},
		SupportedTAIList: []TAI{
			{
				PLMN: PLMN{MCC: "208", MNC: "93"},
				TAC:  "000001",
			},
			{
				PLMN: PLMN{MCC: "208", MNC: "93"},
				TAC:  "000002",
			},
		},
		SupportedPLMNs: []PLMN{
			{MCC: "208", MNC: "93"},
		},
		SupportedSlices: map[string][]SNSSAI{
			"208-93-000001": {
				{SST: 1, SD: "010203"},
				{SST: 1, SD: "112233"},
			},
			"208-93-000002": {
				{SST: 1, SD: "010203"},
				{SST: 1, SD: "112233"},
			},
		},
	}
	return m.config
}

// LoadRestrictiveConfig 載入限制性配置 (只支援一個切片)
func (m *TestConfigManager) LoadRestrictiveConfig() *TestAMFConfig {
	m.config = &TestAMFConfig{
		AMFName: "TestAMF-Restrictive",
		ServedGUAMIList: []models.Guami{
			{
				PlmnId: &models.PlmnIdNid{
					Mcc: "208",
					Mnc: "93",
				},
				AmfId: "cafe00",
			},
		},
		SupportedTAIList: []TAI{
			{
				PLMN: PLMN{MCC: "208", MNC: "93"},
				TAC:  "000001",
			},
		},
		SupportedPLMNs: []PLMN{
			{MCC: "208", MNC: "93"},
		},
		SupportedSlices: map[string][]SNSSAI{
			"208-93-000001": {
				{SST: 1, SD: "010203"}, // 只支援一個切片
			},
		},
	}
	return m.config
}

// LoadMinimalConfig 載入最小配置
func (m *TestConfigManager) LoadMinimalConfig() *TestAMFConfig {
	m.config = &TestAMFConfig{
		AMFName: "TestAMF-Minimal",
		ServedGUAMIList: []models.Guami{
			{
				PlmnId: &models.PlmnIdNid{
					Mcc: "001",
					Mnc: "01",
				},
				AmfId: "000000",
			},
		},
		SupportedTAIList: []TAI{
			{
				PLMN: PLMN{MCC: "001", MNC: "01"},
				TAC:  "000001",
			},
		},
		SupportedPLMNs: []PLMN{
			{MCC: "001", MNC: "01"},
		},
		SupportedSlices: map[string][]SNSSAI{
			"001-01-000001": {
				{SST: 1, SD: "000001"},
			},
		},
	}
	return m.config
}

// SetSupportedSlices 設定支援的切片
func (m *TestConfigManager) SetSupportedSlices(plmn PLMN, tac string, slices []SNSSAI) {
	key := plmn.MCC + "-" + plmn.MNC + "-" + tac
	m.config.SupportedSlices[key] = slices
}

// AddSupportedTAI 新增支援的 TAI
func (m *TestConfigManager) AddSupportedTAI(tai TAI) {
	m.config.SupportedTAIList = append(m.config.SupportedTAIList, tai)
}

// AddSupportedPLMN 新增支援的 PLMN
func (m *TestConfigManager) AddSupportedPLMN(plmn PLMN) {
	m.config.SupportedPLMNs = append(m.config.SupportedPLMNs, plmn)
}

// GetConfig 取得當前配置
func (m *TestConfigManager) GetConfig() *TestAMFConfig {
	return m.config
}

// IsSliceSupported 檢查切片是否被支援
func (m *TestConfigManager) IsSliceSupported(plmn PLMN, tac string, slice SNSSAI) bool {
	key := plmn.MCC + "-" + plmn.MNC + "-" + tac
	slices, exists := m.config.SupportedSlices[key]
	if !exists {
		return false
	}

	for _, s := range slices {
		if s.SST == slice.SST && s.SD == slice.SD {
			return true
		}
	}
	return false
}

// IsPLMNSupported 檢查 PLMN 是否被支援
func (m *TestConfigManager) IsPLMNSupported(plmn PLMN) bool {
	for _, p := range m.config.SupportedPLMNs {
		if p.MCC == plmn.MCC && p.MNC == plmn.MNC {
			return true
		}
	}
	return false
}

// IsTACSupported 檢查 TAC 是否被支援
func (m *TestConfigManager) IsTACSupported(plmn PLMN, tac string) bool {
	for _, tai := range m.config.SupportedTAIList {
		if tai.PLMN.MCC == plmn.MCC && tai.PLMN.MNC == plmn.MNC && tai.TAC == tac {
			return true
		}
	}
	return false
}
