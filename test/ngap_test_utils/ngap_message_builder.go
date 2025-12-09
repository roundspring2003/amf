package ngap_test_utils

import (
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

// NGAPMessageBuilder 提供建構各種 NGAP 訊息的方法
type NGAPMessageBuilder struct{}

// NewNGAPMessageBuilder 建立一個新的 NGAP 訊息建構器
func NewNGAPMessageBuilder() *NGAPMessageBuilder {
	return &NGAPMessageBuilder{}
}

// BuildNGSetupRequest 建立標準的 NG Setup Request
// 參考: free5GC amf/internal/ngap/message/build.go 的寫法
func (b *NGAPMessageBuilder) BuildNGSetupRequest(params NGSetupRequestParams) *ngapType.NGAPPDU {
	var pdu ngapType.NGAPPDU
	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)

	initiatingMessage := pdu.InitiatingMessage
	initiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodeNGSetup
	initiatingMessage.Criticality.Value = ngapType.CriticalityPresentReject
	initiatingMessage.Value.Present = ngapType.InitiatingMessagePresentNGSetupRequest
	initiatingMessage.Value.NGSetupRequest = new(ngapType.NGSetupRequest)

	nGSetupRequest := initiatingMessage.Value.NGSetupRequest
	nGSetupRequestIEs := &nGSetupRequest.ProtocolIEs

	// 1. Global RAN Node ID (mandatory)
	ie := ngapType.NGSetupRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDGlobalRANNodeID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.NGSetupRequestIEsPresentGlobalRANNodeID
	ie.Value.GlobalRANNodeID = new(ngapType.GlobalRANNodeID)

	ie.Value.GlobalRANNodeID.Present = ngapType.GlobalRANNodeIDPresentGlobalGNBID
	ie.Value.GlobalRANNodeID.GlobalGNBID = new(ngapType.GlobalGNBID)
	ie.Value.GlobalRANNodeID.GlobalGNBID.PLMNIdentity = ngapType.PLMNIdentity{
		Value: aper.OctetString("\x02\xf8\x39"), // MCC=208, MNC=93
	}
	ie.Value.GlobalRANNodeID.GlobalGNBID.GNBID.Present = ngapType.GNBIDPresentGNBID
	ie.Value.GlobalRANNodeID.GlobalGNBID.GNBID.GNBID = new(aper.BitString)
	ie.Value.GlobalRANNodeID.GlobalGNBID.GNBID.GNBID.Bytes = []byte{0x00, 0x00, 0x01}
	ie.Value.GlobalRANNodeID.GlobalGNBID.GNBID.GNBID.BitLength = 24

	nGSetupRequestIEs.List = append(nGSetupRequestIEs.List, ie)

	// 2. RAN Node Name (optional)
	if params.RANNodeName != "" {
		ie = ngapType.NGSetupRequestIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDRANNodeName
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.NGSetupRequestIEsPresentRANNodeName
		ie.Value.RANNodeName = new(ngapType.RANNodeName)
		ie.Value.RANNodeName.Value = params.RANNodeName
		nGSetupRequestIEs.List = append(nGSetupRequestIEs.List, ie)
	}

	// 3. Supported TA List (mandatory)
	ie = ngapType.NGSetupRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDSupportedTAList
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.NGSetupRequestIEsPresentSupportedTAList
	ie.Value.SupportedTAList = new(ngapType.SupportedTAList)

	// 建立 Supported TA Items
	for _, tai := range params.SupportedTAIList {
		taItem := ngapType.SupportedTAItem{}
		taItem.TAC = TACToNgapType(tai.TAI.TAC)
		taItem.BroadcastPLMNList.List = make([]ngapType.BroadcastPLMNItem, 0)

		// 建立 Broadcast PLMN Items
		for _, bcastPLMN := range tai.BroadcastPLMNList {
			plmnItem := ngapType.BroadcastPLMNItem{}
			plmnItem.PLMNIdentity = PLMNToNgapType(bcastPLMN.PLMN)
			plmnItem.TAISliceSupportList.List = make([]ngapType.SliceSupportItem, 0)

			// 建立 Slice Support Items
			for _, slice := range bcastPLMN.SliceSupportList {
				sliceItem := ngapType.SliceSupportItem{}
				sliceItem.SNSSAI = SNSSAIToNgapType(slice)
				plmnItem.TAISliceSupportList.List = append(
					plmnItem.TAISliceSupportList.List,
					sliceItem,
				)
			}

			taItem.BroadcastPLMNList.List = append(
				taItem.BroadcastPLMNList.List,
				plmnItem,
			)
		}

		ie.Value.SupportedTAList.List = append(
			ie.Value.SupportedTAList.List,
			taItem,
		)
	}

	nGSetupRequestIEs.List = append(nGSetupRequestIEs.List, ie)

	// 4. Paging DRX (optional)
	if params.PagingDRX != nil {
		ie = ngapType.NGSetupRequestIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDDefaultPagingDRX
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.NGSetupRequestIEsPresentDefaultPagingDRX
		ie.Value.DefaultPagingDRX = new(ngapType.PagingDRX)
		ie.Value.DefaultPagingDRX.Value = *params.PagingDRX
		nGSetupRequestIEs.List = append(nGSetupRequestIEs.List, ie)
	}

	return &pdu
}

// BuildInvalidNGSetupRequest 建立無效的 NG Setup Request (用於負面測試)
func (b *NGAPMessageBuilder) BuildInvalidNGSetupRequest(invalidType string) *ngapType.NGAPPDU {
	switch invalidType {
	case "empty_tai_list":
		// 空的 Supported TA List
		params := NGSetupRequestParams{
			GlobalRANNodeID:  "test-gnb",
			RANNodeName:      "test",
			SupportedTAIList: []SupportedTAI{}, // 空列表
		}
		return b.BuildNGSetupRequest(params)

	case "missing_mandatory_ie":
		// 缺少 Global RAN Node ID
		var pdu ngapType.NGAPPDU
		pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
		pdu.InitiatingMessage = new(ngapType.InitiatingMessage)
		pdu.InitiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodeNGSetup
		pdu.InitiatingMessage.Criticality.Value = ngapType.CriticalityPresentReject
		pdu.InitiatingMessage.Value.Present = ngapType.InitiatingMessagePresentNGSetupRequest
		pdu.InitiatingMessage.Value.NGSetupRequest = new(ngapType.NGSetupRequest)
		// 空的 ProtocolIEs List - 缺少必要 IE
		pdu.InitiatingMessage.Value.NGSetupRequest.ProtocolIEs.List = []ngapType.NGSetupRequestIEs{}
		return &pdu

	default:
		// 返回標準訊息
		return b.BuildNGSetupRequest(NGSetupRequestParams{
			GlobalRANNodeID: "test-gnb",
			RANNodeName:     "test",
			SupportedTAIList: []SupportedTAI{
				{
					TAI: TAI{
						PLMN: PLMN{MCC: "208", MNC: "93"},
						TAC:  "000001",
					},
					BroadcastPLMNList: []BroadcastPLMN{
						{
							PLMN: PLMN{MCC: "208", MNC: "93"},
							SliceSupportList: []SNSSAI{
								{SST: 1, SD: "010203"},
							},
						},
					},
				},
			},
		})
	}
}
