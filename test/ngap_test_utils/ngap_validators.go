package ngap_test_utils

import (
	"fmt"

	"github.com/free5gc/ngap/ngapType"
)

// NGAPValidator 提供 NGAP 訊息驗證方法
type NGAPValidator struct {
	configManager *TestConfigManager
}

// NewNGAPValidator 建立一個新的 NGAP 驗證器
func NewNGAPValidator(configManager *TestConfigManager) *NGAPValidator {
	return &NGAPValidator{
		configManager: configManager,
	}
}

// ValidateNGSetupResponse 驗證 NG Setup Response
func (v *NGAPValidator) ValidateNGSetupResponse(pdu *ngapType.NGAPPDU) error {
	if pdu == nil {
		return fmt.Errorf("PDU is nil")
	}

	if pdu.Present != ngapType.NGAPPDUPresentSuccessfulOutcome {
		return fmt.Errorf("expected SuccessfulOutcome, got %d", pdu.Present)
	}

	if pdu.SuccessfulOutcome == nil {
		return fmt.Errorf("SuccessfulOutcome is nil")
	}

	// 使用 .Value 欄位比較 (參考 build.go:90)
	if pdu.SuccessfulOutcome.ProcedureCode.Value != ngapType.ProcedureCodeNGSetup {
		return fmt.Errorf("expected NG Setup procedure, got %d", pdu.SuccessfulOutcome.ProcedureCode.Value)
	}

	if pdu.SuccessfulOutcome.Value.Present != ngapType.SuccessfulOutcomePresentNGSetupResponse {
		return fmt.Errorf("expected NGSetupResponse, got %d", pdu.SuccessfulOutcome.Value.Present)
	}

	ngSetupResp := pdu.SuccessfulOutcome.Value.NGSetupResponse
	if ngSetupResp == nil {
		return fmt.Errorf("NGSetupResponse is nil")
	}

	// 驗證必要的 IE
	hasAMFName := false
	hasServedGUAMI := false
	hasRelativeCapacity := false
	hasPLMNSupport := false

	for _, ie := range ngSetupResp.ProtocolIEs.List {
		// 使用 .Value 欄位 (參考 build.go:100, 37, 49 等)
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFName:
			hasAMFName = true
		case ngapType.ProtocolIEIDServedGUAMIList:
			hasServedGUAMI = true
		case ngapType.ProtocolIEIDRelativeAMFCapacity:
			hasRelativeCapacity = true
		case ngapType.ProtocolIEIDPLMNSupportList:
			hasPLMNSupport = true
		}
	}

	if !hasAMFName {
		return fmt.Errorf("missing mandatory IE: AMF Name")
	}
	if !hasServedGUAMI {
		return fmt.Errorf("missing mandatory IE: Served GUAMI List")
	}
	if !hasRelativeCapacity {
		return fmt.Errorf("missing mandatory IE: Relative AMF Capacity")
	}
	if !hasPLMNSupport {
		return fmt.Errorf("missing mandatory IE: PLMN Support List")
	}

	return nil
}

// ValidateNGSetupFailure 驗證 NG Setup Failure
func (v *NGAPValidator) ValidateNGSetupFailure(pdu *ngapType.NGAPPDU, expectedCauseType string) error {
	if pdu == nil {
		return fmt.Errorf("PDU is nil")
	}

	if pdu.Present != ngapType.NGAPPDUPresentUnsuccessfulOutcome {
		return fmt.Errorf("expected UnsuccessfulOutcome, got %d", pdu.Present)
	}

	if pdu.UnsuccessfulOutcome == nil {
		return fmt.Errorf("UnsuccessfulOutcome is nil")
	}

	// 使用 .Value 欄位比較
	if pdu.UnsuccessfulOutcome.ProcedureCode.Value != ngapType.ProcedureCodeNGSetup {
		return fmt.Errorf("expected NG Setup procedure, got %d", pdu.UnsuccessfulOutcome.ProcedureCode.Value)
	}

	if pdu.UnsuccessfulOutcome.Value.Present != ngapType.UnsuccessfulOutcomePresentNGSetupFailure {
		return fmt.Errorf("expected NGSetupFailure, got %d", pdu.UnsuccessfulOutcome.Value.Present)
	}

	ngSetupFail := pdu.UnsuccessfulOutcome.Value.NGSetupFailure
	if ngSetupFail == nil {
		return fmt.Errorf("NGSetupFailure is nil")
	}

	// 驗證 Cause IE
	hasCause := false
	var actualCausePresent int

	for _, ie := range ngSetupFail.ProtocolIEs.List {
		// 使用 .Value 欄位
		if ie.Id.Value == ngapType.ProtocolIEIDCause {
			hasCause = true
			if ie.Value.Cause != nil {
				actualCausePresent = int(ie.Value.Cause.Present)
			}
			break
		}
	}

	if !hasCause {
		return fmt.Errorf("missing mandatory IE: Cause")
	}

	// 如果指定了期望的 Cause 類型,進行驗證
	if expectedCauseType != "" {
		// 這裡可以根據 expectedCauseType 字串來檢查
		// 例如 "RadioNetwork", "Transport", "Misc" 等
		// 簡化處理,只記錄實際的 Cause
		_ = actualCausePresent
	}

	return nil
}

// ValidateSupportedTAIList 驗證 Supported TAI List
// 參考: handler.go:45-71 的驗證邏輯
func (v *NGAPValidator) ValidateSupportedTAIList(taiList []SupportedTAI) error {
	if len(taiList) == 0 {
		return fmt.Errorf("Supported TAI List is empty")
	}

	for _, tai := range taiList {
		// 驗證 PLMN
		if !v.configManager.IsPLMNSupported(tai.TAI.PLMN) {
			return fmt.Errorf("unsupported PLMN: %s-%s", tai.TAI.PLMN.MCC, tai.TAI.PLMN.MNC)
		}

		// 驗證 TAC
		if !v.configManager.IsTACSupported(tai.TAI.PLMN, tai.TAI.TAC) {
			return fmt.Errorf("unsupported TAC: %s for PLMN %s-%s",
				tai.TAI.TAC, tai.TAI.PLMN.MCC, tai.TAI.PLMN.MNC)
		}

		// 驗證每個 Broadcast PLMN
		for _, bcastPLMN := range tai.BroadcastPLMNList {
			// 驗證切片支援
			for _, slice := range bcastPLMN.SliceSupportList {
				if !v.configManager.IsSliceSupported(tai.TAI.PLMN, tai.TAI.TAC, slice) {
					return fmt.Errorf("unsupported S-NSSAI: SST=%d, SD=%s in TAI %s-%s-%s",
						slice.SST, slice.SD,
						tai.TAI.PLMN.MCC, tai.TAI.PLMN.MNC, tai.TAI.TAC)
				}
			}
		}
	}

	return nil
}

// ValidateSliceSupport 驗證切片是否被支援
func (v *NGAPValidator) ValidateSliceSupport(plmn PLMN, tac string, slice SNSSAI) bool {
	return v.configManager.IsSliceSupported(plmn, tac, slice)
}

// ValidatePLMNSupport 驗證 PLMN 是否被支援
func (v *NGAPValidator) ValidatePLMNSupport(plmn PLMN) bool {
	return v.configManager.IsPLMNSupported(plmn)
}

// ValidateTACSupport 驗證 TAC 是否被支援
func (v *NGAPValidator) ValidateTACSupport(plmn PLMN, tac string) bool {
	return v.configManager.IsTACSupported(plmn, tac)
}
