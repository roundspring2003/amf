package ngap_test_utils

import (
	"fmt"
	"net"

	"github.com/free5gc/ngap/ngapType"
)

// FakeGNB 模擬一個 gNB 用於測試
type FakeGNB struct {
	Conn             net.Conn
	GlobalRANNodeID  string
	RANNodeName      string
	SupportedTAIList []SupportedTAI
	MessageBuilder   *NGAPMessageBuilder
}

// NewFakeGNB 建立一個新的 Fake gNB
func NewFakeGNB(conn net.Conn, ranNodeName string) *FakeGNB {
	return &FakeGNB{
		Conn:           conn,
		GlobalRANNodeID: "test-gnb-" + ranNodeName,
		RANNodeName:    ranNodeName,
		MessageBuilder: NewNGAPMessageBuilder(),
	}
}

// SetSupportedSlices 設定支援的網路切片
func (g *FakeGNB) SetSupportedSlices(plmn PLMN, tac string, slices []SNSSAI) {
	tai := SupportedTAI{
		TAI: TAI{
			PLMN: plmn,
			TAC:  tac,
		},
		BroadcastPLMNList: []BroadcastPLMN{
			{
				PLMN:             plmn,
				SliceSupportList: slices,
			},
		},
	}
	g.SupportedTAIList = append(g.SupportedTAIList, tai)
}

// SendNGSetupRequest 發送 NG Setup Request
func (g *FakeGNB) SendNGSetupRequest() (*ngapType.NGAPPDU, error) {
	params := NGSetupRequestParams{
		GlobalRANNodeID:  g.GlobalRANNodeID,
		RANNodeName:      g.RANNodeName,
		SupportedTAIList: g.SupportedTAIList,
	}

	pdu := g.MessageBuilder.BuildNGSetupRequest(params)

	// 在真實場景中,這裡會編碼並發送訊息
	// 現在只是返回建立的 PDU 用於測試

	return pdu, nil
}

// SendInvalidNGSetupRequest 發送無效的 NG Setup Request (用於負面測試)
func (g *FakeGNB) SendInvalidNGSetupRequest(invalidType string) (*ngapType.NGAPPDU, error) {
	pdu := g.MessageBuilder.BuildInvalidNGSetupRequest(invalidType)
	return pdu, nil
}

// ExpectNGSetupResponse 期待收到 NG Setup Response
func (g *FakeGNB) ExpectNGSetupResponse() error {
	// 在真實場景中,這裡會接收並解碼訊息
	// 現在只是一個佔位符
	return nil
}

// ExpectNGSetupFailure 期待收到 NG Setup Failure
// expectedCause: 期待的失敗原因 (如 "No Network Slices Available")
func (g *FakeGNB) ExpectNGSetupFailure(expectedCause string) error {
	// 在真實場景中,這裡會接收並驗證失敗訊息
	// 現在只是一個佔位符
	return nil
}

// Close 關閉連接
func (g *FakeGNB) Close() error {
	if g.Conn != nil {
		return g.Conn.Close()
	}
	return nil
}

// String 返回 gNB 的字串表示
func (g *FakeGNB) String() string {
	return fmt.Sprintf("FakeGNB{Name: %s, TAIs: %d}", g.RANNodeName, len(g.SupportedTAIList))
}
