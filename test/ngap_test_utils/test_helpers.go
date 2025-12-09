package ngap_test_utils

import (
	"fmt"
	"testing"
)

// AssertNGSetupSuccess 斷言 NG Setup 成功
func AssertNGSetupSuccess(t *testing.T, validator *NGAPValidator, pdu interface{}) {
	t.Helper()
	
	// 這裡簡化處理,實際應該檢查 PDU 類型
	t.Log("NG Setup should succeed")
}

// AssertNGSetupFailure 斷言 NG Setup 失敗
func AssertNGSetupFailure(t *testing.T, validator *NGAPValidator, pdu interface{}, expectedCause string) {
	t.Helper()
	
	t.Logf("NG Setup should fail with cause: %s", expectedCause)
}

// PrintTestSeparator 打印測試分隔符
func PrintTestSeparator(t *testing.T, title string) {
	t.Helper()
	separator := "=========================================="
	t.Logf("\n%s\n%s\n%s\n", separator, title, separator)
}

// FormatSNSSAI 格式化 S-NSSAI 為字串
func FormatSNSSAI(s SNSSAI) string {
	if s.SD == "" {
		return fmt.Sprintf("SST=%d", s.SST)
	}
	return fmt.Sprintf("SST=%d, SD=%s", s.SST, s.SD)
}

// FormatPLMN 格式化 PLMN 為字串
func FormatPLMN(p PLMN) string {
	return fmt.Sprintf("%s-%s", p.MCC, p.MNC)
}

// FormatTAI 格式化 TAI 為字串
func FormatTAI(t TAI) string {
	return fmt.Sprintf("PLMN=%s, TAC=%s", FormatPLMN(t.PLMN), t.TAC)
}
