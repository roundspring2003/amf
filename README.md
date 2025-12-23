# free5GC AMF 測試指南

本文件說明如何執行 free5GC AMF 模組的各項測試。測試涵蓋單元測試、整合測試、併發測試及 NGAP 協議合規性測試。

---

## 目錄

1. [環境需求](#環境需求)
2. [測試架構概覽](#測試架構概覽)
3. [快速開始](#快速開始)
4. [測試分類詳解](#測試分類詳解)
   - [Context 模組測試](#context-模組測試)
   - [SBI API 測試](#sbi-api-測試)
   - [NGAP 協議合規性測試](#ngap-協議合規性測試)
5. [測試執行指令](#測試執行指令)
6. [測試覆蓋率](#測試覆蓋率)
7. [常見問題](#常見問題)

---

## 環境需求

- **Go 版本**: 1.21+
- **測試框架**: Go testing package
- **斷言庫**: 
  - `github.com/stretchr/testify/require`
  - `github.com/stretchr/testify/assert`
  - `github.com/smartystreets/goconvey`
- **測試目標**: free5GC AMF v4.1.0

### 安裝依賴

```bash
go mod download
```

---

## 測試架構概覽

```
amf/
├── internal/
│   ├── context/                    # Context 模組測試
│   │   ├── amf_ran_test.go        # RAN 併發測試
│   │   ├── amf_ue_cm_state_test.go       # UE CM 狀態測試
│   │   ├── amf_ue_ranue_lifecycle_test.go # UE 生命週期測試
│   │   ├── ran_ue_update_location_test.go # 位置更新測試
│   │   ├── concurrent_stress_test.go      # 併發壓力測試
│   │   ├── context_test.go               # Context 核心功能測試
│   │   └── timer_test.go                 # Timer 測試
│   └── sbi/                        # SBI API 測試
│       ├── server_test.go         # Server 測試框架
│       ├── api_communication_test.go
│       ├── api_eventexposure_test.go
│       ├── api_httpcallback_test.go
│       ├── api_location_test.go
│       ├── api_mbsbroadcast_test.go
│       ├── api_mbscommunication_test.go
│       ├── api_mt_test.go
│       └── api_oam_test.go
└── test/
    ├── ngap_protocol_compliance/   # NGAP 協議合規性測試
    │   ├── ng_setup_compliance_test.go
    │   └── ng_setup_investigation_test.go
    └── ngap_test_utils/           # 測試工具函式庫
        ├── fake_gnb.go
        ├── ngap_message_builder.go
        ├── ngap_validators.go
        ├── test_config_manager.go
        ├── test_helpers.go
        └── test_types.go
```

---

## 快速開始

### 執行所有測試

```bash
cd ~/free5gc/NFs/amf
go test ./... -v
```

### 執行特定模組測試

```bash
# Context 模組
go test -v ./internal/context/...

# SBI API 模組
go test -v ./internal/sbi/...

# NGAP 協議合規性
go test -v ./test/ngap_protocol_compliance/...
```

---

## 測試分類詳解

### Context 模組測試

位於 `internal/context/` 目錄，涵蓋 AMF 核心功能。

#### 1. CM-State 測試 (`amf_ue_cm_state_test.go`)

測試 UE 連線狀態管理 (CM-IDLE / CM-CONNECTED)。

| 測試案例 | 說明 |
|---------|------|
| `TestAmfUeCmState_InitialState` | 驗證新 UE 初始為 CM-IDLE |
| `TestAmfUeCmState_AfterRanUeAttach` | 驗證連接後轉為 CM-CONNECTED |
| `TestAmfUeCmState_MultiAccessType` | 驗證 3GPP/Non-3GPP 狀態獨立 |
| `TestAmfUeCmState_AfterRanUeDetach` | 驗證分離後恢復 CM-IDLE |
| `TestAmfUeCmState_BothAccessTypes` | 測試雙 Access Type 場景 |
| `TestAmfUeCmState_NilRanUe` | 邊界測試: RanUe 為 nil |

```bash
go test -v -run TestAmfUeCmState ./internal/context/
```

#### 2. UE 生命週期測試 (`amf_ue_ranue_lifecycle_test.go`)

測試 AmfUe 和 RanUe 之間的連接生命週期。

| 測試案例 | 說明 |
|---------|------|
| `TestAmfUeRanUe_AttachDetachLifecycle` | 完整 Attach/Detach 流程 |
| `TestAmfUeRanUe_MultipleAttachDetach` | 多次循環穩定性 |
| `TestAmfUeRanUe_BothAccessTypes` | 雙 Access Type 操作 |
| `TestAmfUeRanUe_DetachWithoutAttach` | 錯誤處理: 未 Attach 直接 Detach |

```bash
go test -v -run TestAmfUeRanUe ./internal/context/
```

#### 3. 位置更新測試 (`ran_ue_update_location_test.go`)

測試 NGAP UserLocationInformation 解析。

| 測試案例 | 說明 |
|---------|------|
| `TestRanUe_UpdateLocation_NR` | 5G NR 位置資訊解析 |
| `TestRanUe_UpdateLocation_SameTAC` | 相同 TAC 行為 |
| `TestRanUe_UpdateLocation_EUTRA` | 4G LTE 位置解析 |
| `TestRanUe_UpdateLocation_NilInput` | nil 輸入容錯 |

```bash
go test -v -run TestRanUe_UpdateLocation ./internal/context/
```

#### 4. 併發壓力測試 (`concurrent_stress_test.go`)

測試 sync.Map 在高併發下的執行緒安全性。

| 測試案例 | 說明 |
|---------|------|
| `TestAMFContext_ConcurrentRanUePool` | 10000 個 RanUe 併發刪除 |
| `TestAMFContext_ConcurrentUePool` | UePool CRUD 併發操作 |
| `TestAmfRan_ConcurrentRanUeListStressTest` | 極限壓力測試 |
| `TestAMFContext_MixedConcurrentOperations` | 混合併發操作 |
| `TestConcurrentMapOperations_EdgeCases` | 邊界情況 |

```bash
go test -v -run Concurrent ./internal/context/
```

#### 5. Context 核心功能測試 (`context_test.go`)

| 測試案例 | 說明 |
|---------|------|
| `TestGetIntAlgOrder_Mapping` | 演算法映射 |
| `TestAmfUe_Operations` | UE 操作 (建立/查找/重複) |
| `TestAmfRan_Operations` | RAN 操作 |
| `TestRanUe_SwitchToRan` | RAN 切換 |
| `TestTmsiGenerator_Lifecycle` | TMSI 分配/釋放 |

```bash
go test -v ./internal/context/context_test.go
```

#### 6. RAN 併發測試 (`amf_ran_test.go`)

```bash
go test -v -run TestRemoveAndRemoveAllRanUeRaceCondition ./internal/context/
```

---

### SBI API 測試

位於 `internal/sbi/` 目錄，測試 HTTP API 端點。

#### 測試框架

所有 SBI 測試使用共用的 Mock 框架 (`server_test.go`):

```go
// 建立測試 Server
s, ctx := NewTestServer(t)

// 管理測試 UE (自動清理)
ManageTestUE(t, fakeUe)

// 執行 JSON 請求
w := PerformJSONRequest(router, http.MethodPost, "/path", jsonBody)
```

#### Communication API (`api_communication_test.go`)

測試 Namf_Communication 服務。

```bash
go test -v -run TestCommunication ./internal/sbi/
```

| 測試類別 | 說明 |
|---------|------|
| `RouteDefinitions` | 驗證 18 個路由定義 |
| `BasicEndpoints` | Health check |
| `NotImplementedEndpoints` | 未實作端點回傳 501 |
| `Subscription_ErrorCases` | 訂閱錯誤處理 |
| `UEContext_ErrorCases` | UE Context 錯誤處理 |
| `N1N2Message_ErrorCases` | N1N2 訊息錯誤處理 |

#### Event Exposure API (`api_eventexposure_test.go`)

測試 Namf_EventExposure 服務。

```bash
go test -v -run TestEventExposure ./internal/sbi/
```

| 測試類別 | 說明 |
|---------|------|
| `ModifySubscription` | 修改訂閱 (錯誤/成功) |
| `CreateSubscription` | 建立訂閱 |
| `DeleteSubscription` | 刪除訂閱 |

#### HTTP Callback API (`api_httpcallback_test.go`)

測試回呼端點。

```bash
go test -v -run TestCallback ./internal/sbi/
go test -v -run TestHTTPAmPolicyControl ./internal/sbi/
go test -v -run TestHTTPSmContextStatus ./internal/sbi/
```

| 測試案例 | 說明 |
|---------|------|
| `AmPolicyControlUpdateNotifyUpdate` | Policy 更新通知 |
| `AmPolicyControlUpdateNotifyTerminate` | Policy 終止通知 |
| `SmContextStatusNotify` | SM Context 狀態通知 |
| `N1MessageNotify` | N1 訊息通知 (Multipart) |
| `DeregistrationNotification` | 註銷通知 |

#### Location API (`api_location_test.go`)

```bash
go test -v -run TestLocation ./internal/sbi/
```

#### MT API (`api_mt_test.go`)

```bash
go test -v -run TestMT ./internal/sbi/
go test -v -run TestHTTPProvideDomainSelectionInfo ./internal/sbi/
```

#### OAM API (`api_oam_test.go`)

```bash
go test -v -run TestOAM ./internal/sbi/
go test -v -run TestHTTPRegisteredUEContext ./internal/sbi/
```

#### MBS APIs (`api_mbsbroadcast_test.go`, `api_mbscommunication_test.go`)

```bash
go test -v -run TestMbs ./internal/sbi/
```

---

### NGAP 協議合規性測試

位於 `test/ngap_protocol_compliance/` 目錄，驗證 NGAP 訊息處理。

#### NG Setup 合規性測試 (`ng_setup_compliance_test.go`)

| 測試案例 | 說明 |
|---------|------|
| `TestNGSetup_StandardConfiguration` | 標準配置 NG Setup |
| `TestNGSetup_UnsupportedSlice` | 不支援的切片 (Bug 重現) |
| `TestNGSetup_MixedSlices` | 部分支援的切片 |
| `TestNGSetup_UnsupportedPLMN` | 不支援的 PLMN |
| `TestNGSetup_EmptyTAIList` | 空 TAI 列表 |
| `TestNGSetup_UnsupportedTAC` | 不支援的 TAC |
| `TestNGSetup_InvalidTACFormat` | 無效 TAC 格式 |
| `TestNGSetup_MultipleTAIsWithPartialSupport` | 部分支援的多 TAI |
| `TestNGSetup_MaximumNumberOfSlices` | 最大切片數量 |
| `TestNGSetup_ExceedMaximumSlices` | 超過最大切片限制 |
| `TestNGSetup_DuplicateSlicesInSameTAI` | 重複切片 |
| `TestNGSetup_VeryLongRANNodeName` | 超長 RAN Node Name |
| `TestNGSetup_InvalidGlobalRANNodeID` | 無效 Global RAN Node ID |
| `TestNGSetup_MalformedPLMNID` | 畸形 PLMN ID |
| `TestNGSetup_InvalidSliceConfiguration` | 無效切片配置 |
| `TestNGSetup_RapidRepeatedRequests` | 快速重複請求 |
| `TestNGSetup_ConcurrentFromMultipleGNBs` | 多 gNB 併發 |

```bash
go test -v ./test/ngap_protocol_compliance/
```

#### NG Setup 調查測試 (`ng_setup_investigation_test.go`)

用於調查 AMF 驗證策略的深度測試。

| 測試案例 | 說明 |
|---------|------|
| `TestNGSetup_Investigation_MultiTAIValidationStrategy` | 多 TAI 驗證策略調查 |
| `TestNGSetup_Investigation_AllSupportedVsAtLeastOne` | "全部支援" vs "至少一個" 策略 |

```bash
go test -v -run Investigation ./test/ngap_protocol_compliance/
```

---

## 測試執行指令

### 基本指令

```bash
# 執行所有測試
go test ./... -v

# 執行特定測試
go test -v -run TestName ./path/to/package/

# 執行測試並顯示覆蓋率
go test -v -cover ./...

# 生成覆蓋率報告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# 檢測 Race Condition
go test -race -v ./...

# 設定測試超時
go test -timeout 30s ./...

# 只執行短測試
go test -short ./...
```

### 按模組執行

```bash
# Context 所有測試
go test -v ./internal/context/...

# SBI 所有測試
go test -v ./internal/sbi/...

# NGAP 協議測試
go test -v ./test/ngap_protocol_compliance/...

# 僅併發測試
go test -v -run Concurrent ./internal/context/

# 僅壓力測試
go test -v -run StressTest ./internal/context/
```

### 按測試類型執行

```bash
# 所有 CM State 測試
go test -v -run TestAmfUeCmState ./internal/context/

# 所有生命週期測試
go test -v -run TestAmfUeRanUe ./internal/context/

# 所有位置更新測試
go test -v -run TestRanUe_UpdateLocation ./internal/context/

# 所有 NG Setup 測試
go test -v -run TestNGSetup ./test/ngap_protocol_compliance/
```

---

## 測試工具函式庫

### NGAP 測試工具 (`test/ngap_test_utils/`)

#### FakeGNB (`fake_gnb.go`)

模擬 gNB 用於測試。

```go
fakeConn := &utils.FakeNetConn{}
gNB := utils.NewFakeGNB(fakeConn, "test-gnb-1")

gNB.SetSupportedSlices(
    utils.PLMN{MCC: "208", MNC: "93"},
    "000001",
    []utils.SNSSAI{{SST: 1, SD: "010203"}},
)

pdu, err := gNB.SendNGSetupRequest()
```

#### TestConfigManager (`test_config_manager.go`)

管理測試用 AMF 配置。

```go
configManager := utils.NewTestConfigManager()
config := configManager.LoadStandardConfig()

// 檢查支援
configManager.IsSliceSupported(plmn, tac, slice)
configManager.IsPLMNSupported(plmn)
configManager.IsTACSupported(plmn, tac)
```

#### NGAPValidator (`ngap_validators.go`)

驗證 NGAP 訊息。

```go
validator := utils.NewNGAPValidator(configManager)
err := validator.ValidateSupportedTAIList(taiList)
err := validator.ValidateNGSetupResponse(pdu)
err := validator.ValidateNGSetupFailure(pdu, expectedCause)
```

### SBI 測試工具 (`internal/sbi/server_test.go`)

```go
// 建立測試 Server
s, ctx := NewTestServer(t)

// 管理測試 UE
ManageTestUE(t, fakeUe)

// 執行 HTTP 請求
w := PerformJSONRequest(router, method, url, body)
```
---

## 相關文件

- [free5GC 官方文件](https://free5gc.org/)
- [3GPP TS 23.502 - 5G System Procedures](https://www.3gpp.org/DynaReport/23502.htm)
- [NGAP Protocol Specification](https://www.3gpp.org/DynaReport/38413.htm)
- [Go Testing Package](https://pkg.go.dev/testing)
- [Testify Documentation](https://github.com/stretchr/testify)

---

## 貢獻指南

1. 新測試應遵循 3A 原則 (Arrange-Act-Assert)
2. 使用 `t.Parallel()` 標記可並行的測試
3. 使用 `t.Cleanup()` 確保資源釋放
4. 測試命名: `Test<Function>_<Scenario>`
5. 邊界測試: nil、空值、極端情況

