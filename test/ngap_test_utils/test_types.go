package ngap_test_utils

import (
	"net"
	"time"

	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
	"github.com/free5gc/openapi/models"
)

// SNSSAI 代表一個網路切片選擇輔助資訊
type SNSSAI struct {
	SST int32  // Slice/Service Type
	SD  string // Slice Differentiator (hex string, 例如 "010203")
}

// PLMN 代表公共陸地行動網路識別碼
type PLMN struct {
	MCC string // Mobile Country Code (例如 "208")
	MNC string // Mobile Network Code (例如 "93")
}

// TAI 代表追蹤區域識別碼
type TAI struct {
	PLMN PLMN
	TAC  string // Tracking Area Code (hex string, 例如 "000001")
}

// SupportedTAI 代表 gNB 支援的 TAI 和切片
type SupportedTAI struct {
	TAI               TAI
	BroadcastPLMNList []BroadcastPLMN
}

// BroadcastPLMN 代表廣播的 PLMN 和支援的切片
type BroadcastPLMN struct {
	PLMN             PLMN
	SliceSupportList []SNSSAI
}

// NGSetupRequestParams 代表 NG Setup Request 的參數
type NGSetupRequestParams struct {
	GlobalRANNodeID  string
	RANNodeName      string
	SupportedTAIList []SupportedTAI
	PagingDRX        *aper.Enumerated // 使用正確的類型
}

// NGSetupResponseParams 代表 NG Setup Response 的參數
type NGSetupResponseParams struct {
	AMFName             string
	ServedGUAMIList     []models.Guami
	RelativeAMFCapacity int64
	PLMNSupportList     []PLMNSupportItem
}

// PLMNSupportItem 代表 AMF 支援的 PLMN 和切片
type PLMNSupportItem struct {
	PLMN             PLMN
	SliceSupportList []SNSSAI
}

// NGSetupFailureParams 代表 NG Setup Failure 的參數
type NGSetupFailureParams struct {
	Cause                  ngapType.Cause
	TimeToWait             *ngapType.TimeToWait // 使用正確的類型
	CriticalityDiagnostics *ngapType.CriticalityDiagnostics
}

// TestAMFConfig 代表測試用的 AMF 配置
type TestAMFConfig struct {
	AMFName          string
	ServedGUAMIList  []models.Guami
	SupportedTAIList []TAI
	SupportedPLMNs   []PLMN
	SupportedSlices  map[string][]SNSSAI // Key: PLMN-TAC, Value: supported slices
}

// FakeNetConn 實作一個假的網路連接用於測試
type FakeNetConn struct {
	ReadBuffer  []byte
	WriteBuffer []byte
	Closed      bool
}

func (f *FakeNetConn) Read(b []byte) (n int, err error) {
	if f.Closed {
		return 0, net.ErrClosed
	}
	n = copy(b, f.ReadBuffer)
	f.ReadBuffer = f.ReadBuffer[n:]
	return n, nil
}

func (f *FakeNetConn) Write(b []byte) (n int, err error) {
	if f.Closed {
		return 0, net.ErrClosed
	}
	f.WriteBuffer = append(f.WriteBuffer, b...)
	return len(b), nil
}

func (f *FakeNetConn) Close() error {
	f.Closed = true
	return nil
}

func (f *FakeNetConn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 38412}
}

func (f *FakeNetConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.2"), Port: 38412}
}

func (f *FakeNetConn) SetDeadline(t time.Time) error      { return nil }
func (f *FakeNetConn) SetReadDeadline(t time.Time) error  { return nil }
func (f *FakeNetConn) SetWriteDeadline(t time.Time) error { return nil }

// Helper functions for converting between types

// SNSSAIToNgapType 將 SNSSAI 轉換為 ngapType.SNSSAI
func SNSSAIToNgapType(s SNSSAI) ngapType.SNSSAI {
	var result ngapType.SNSSAI

	// SST
	result.SST.Value = aper.OctetString{byte(s.SST)}

	// SD (如果存在)
	if s.SD != "" {
		sd := make([]byte, 3)
		// 將 hex string 轉換為 bytes (例如 "010203" -> {0x01, 0x02, 0x03})
		for i := 0; i < 3 && i*2 < len(s.SD); i++ {
			var b byte
			if i*2+1 < len(s.SD) {
				// 解析兩個 hex 字符
				highNibble := hexCharToByte(s.SD[i*2])
				lowNibble := hexCharToByte(s.SD[i*2+1])
				b = (highNibble << 4) | lowNibble
			}
			sd[i] = b
		}
		result.SD = &ngapType.SD{Value: aper.OctetString(sd)}
	}

	return result
}

// PLMNToNgapType 將 PLMN 轉換為 ngapType.PLMNIdentity
func PLMNToNgapType(p PLMN) ngapType.PLMNIdentity {
	// PLMN ID 編碼 (簡化版本)
	// 實際編碼更複雜,這裡使用簡化版本
	plmnBytes := []byte{0x02, 0xf8, 0x39} // 預設 208-93

	if len(p.MCC) >= 3 && len(p.MNC) >= 2 {
		// MCC digit 1
		mcc1 := hexCharToByte(p.MCC[0])
		// MCC digit 2
		mcc2 := hexCharToByte(p.MCC[1])
		// MCC digit 3
		mcc3 := hexCharToByte(p.MCC[2])
		// MNC digit 1
		mnc1 := hexCharToByte(p.MNC[0])
		// MNC digit 2
		mnc2 := hexCharToByte(p.MNC[1])

		// 編碼: 
		// Byte 1: MCC digit 2 | MCC digit 1
		// Byte 2: MNC digit 1 | MCC digit 3
		// Byte 3: MNC digit 3 | MNC digit 2 (如果是 2 位 MNC, digit 3 = 0xF)
		plmnBytes[0] = (mcc2 << 4) | mcc1
		plmnBytes[1] = (mnc1 << 4) | mcc3

		if len(p.MNC) == 3 {
			mnc3 := hexCharToByte(p.MNC[2])
			plmnBytes[2] = (mnc3 << 4) | mnc2
		} else {
			// 2 位 MNC, 第三位設為 0xF
			plmnBytes[2] = (0xF << 4) | mnc2
		}
	}

	return ngapType.PLMNIdentity{
		Value: aper.OctetString(plmnBytes),
	}
}

// TACToNgapType 將 TAC string 轉換為 ngapType.TAC
func TACToNgapType(tac string) ngapType.TAC {
	// TAC 是 3 bytes (例如 "000001" -> {0x00, 0x00, 0x01})
	tacBytes := make([]byte, 3)
	for i := 0; i < 3 && i*2 < len(tac); i++ {
		if i*2+1 < len(tac) {
			highNibble := hexCharToByte(tac[i*2])
			lowNibble := hexCharToByte(tac[i*2+1])
			tacBytes[i] = (highNibble << 4) | lowNibble
		}
	}
	return ngapType.TAC{
		Value: aper.OctetString(tacBytes),
	}
}

// hexCharToByte 將單個 hex 字符轉換為 byte
func hexCharToByte(c byte) byte {
	switch {
	case '0' <= c && c <= '9':
		return c - '0'
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10
	default:
		return 0
	}
}
