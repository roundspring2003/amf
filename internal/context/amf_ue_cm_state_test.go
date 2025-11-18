package context

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/free5gc/amf/internal/logger"
	"github.com/free5gc/openapi/models"
)

// TestAmfUeCmState_InitialState tests that a newly initialized AmfUe is in CM-IDLE state
// 測試目標：驗證新建立的 AmfUe 初始狀態為 CM-IDLE
func TestAmfUeCmState_InitialState(t *testing.T) {
	// 準備：建立一個 AmfUe 物件並初始化
	ue := &AmfUe{}
	ue.init()

	// 測試 3GPP Access
	t.Run("3GPP Access - Initial CM-IDLE State", func(t *testing.T) {
		// 斷言：初始狀態下應該是 CM-IDLE
		require.True(t, ue.CmIdle(models.AccessType__3_GPP_ACCESS),
			"Newly initialized AmfUe should be in CM-IDLE state for 3GPP access")

		// 斷言：初始狀態下不應該是 CM-CONNECTED
		require.False(t, ue.CmConnect(models.AccessType__3_GPP_ACCESS),
			"Newly initialized AmfUe should NOT be in CM-CONNECTED state for 3GPP access")
	})

	// 測試 Non-3GPP Access
	t.Run("Non-3GPP Access - Initial CM-IDLE State", func(t *testing.T) {
		// 斷言：初始狀態下應該是 CM-IDLE
		require.True(t, ue.CmIdle(models.AccessType_NON_3_GPP_ACCESS),
			"Newly initialized AmfUe should be in CM-IDLE state for Non-3GPP access")

		// 斷言：初始狀態下不應該是 CM-CONNECTED
		require.False(t, ue.CmConnect(models.AccessType_NON_3_GPP_ACCESS),
			"Newly initialized AmfUe should NOT be in CM-CONNECTED state for Non-3GPP access")
	})
}

// TestAmfUeCmState_AfterRanUeAttach tests CM state after attaching a RanUe
// 測試目標：驗證連接 RanUe 後，AmfUe 狀態變為 CM-CONNECTED
func TestAmfUeCmState_AfterRanUeAttach(t *testing.T) {
	// 準備：建立 AmfUe 並初始化
	ue := &AmfUe{}
	ue.init()

	// 驗證初始狀態
	require.True(t, ue.CmIdle(models.AccessType__3_GPP_ACCESS),
		"AmfUe should start in CM-IDLE state")

	t.Run("Attach RanUe - Transition to CM-CONNECTED", func(t *testing.T) {
		// 手動設定 RanUe (模擬連線建立)
		// 建立一個假的 RanUe 物件
		fakeRanUe := &RanUe{
			RanUeNgapId: 1,
			AmfUeNgapId: 1,
			Log:         logger.NgapLog.WithField("test", "cm-state"),
		}

		// 將 RanUe 連接到 AmfUe
		ue.RanUe[models.AccessType__3_GPP_ACCESS] = fakeRanUe

		// 斷言：現在應該是 CM-CONNECTED 狀態
		require.True(t, ue.CmConnect(models.AccessType__3_GPP_ACCESS),
			"AmfUe should be in CM-CONNECTED state after RanUe is attached")

		// 斷言：不應該是 CM-IDLE 狀態
		require.False(t, ue.CmIdle(models.AccessType__3_GPP_ACCESS),
			"AmfUe should NOT be in CM-IDLE state when RanUe is attached")
	})
}

// TestAmfUeCmState_MultiAccessType tests CM state for different access types independently
// 測試目標：驗證不同 Access Type 的 CM 狀態是獨立的
func TestAmfUeCmState_MultiAccessType(t *testing.T) {
	// 準備：建立 AmfUe 並初始化
	ue := &AmfUe{}
	ue.init()

	t.Run("Independent CM State per Access Type", func(t *testing.T) {
		// 只連接 3GPP Access 的 RanUe
		fakeRanUe3gpp := &RanUe{
			RanUeNgapId: 1,
			AmfUeNgapId: 1,
			Log:         logger.NgapLog.WithField("test", "3gpp"),
		}
		ue.RanUe[models.AccessType__3_GPP_ACCESS] = fakeRanUe3gpp

		// 斷言：3GPP Access 應該是 CM-CONNECTED
		require.True(t, ue.CmConnect(models.AccessType__3_GPP_ACCESS),
			"3GPP access should be CM-CONNECTED")
		require.False(t, ue.CmIdle(models.AccessType__3_GPP_ACCESS),
			"3GPP access should NOT be CM-IDLE")

		// 斷言：Non-3GPP Access 應該仍然是 CM-IDLE
		require.True(t, ue.CmIdle(models.AccessType_NON_3_GPP_ACCESS),
			"Non-3GPP access should still be CM-IDLE")
		require.False(t, ue.CmConnect(models.AccessType_NON_3_GPP_ACCESS),
			"Non-3GPP access should NOT be CM-CONNECTED")
	})
}

// TestAmfUeCmState_AfterRanUeDetach tests CM state after detaching RanUe
// 測試目標：驗證 RanUe 分離後，AmfUe 狀態恢復為 CM-IDLE
func TestAmfUeCmState_AfterRanUeDetach(t *testing.T) {
	// 準備：建立 AmfUe 並初始化
	ue := &AmfUe{}
	ue.init()

	// 先連接 RanUe
	fakeRanUe := &RanUe{
		RanUeNgapId: 1,
		AmfUeNgapId: 1,
		Log:         logger.NgapLog.WithField("test", "detach"),
	}
	ue.RanUe[models.AccessType__3_GPP_ACCESS] = fakeRanUe

	// 驗證已連接
	require.True(t, ue.CmConnect(models.AccessType__3_GPP_ACCESS),
		"AmfUe should be CM-CONNECTED after attaching RanUe")

	t.Run("Detach RanUe - Return to CM-IDLE", func(t *testing.T) {
		// 移除 RanUe (模擬連線斷開)
		delete(ue.RanUe, models.AccessType__3_GPP_ACCESS)

		// 斷言：現在應該回到 CM-IDLE 狀態
		require.True(t, ue.CmIdle(models.AccessType__3_GPP_ACCESS),
			"AmfUe should return to CM-IDLE state after RanUe is detached")

		// 斷言：不應該是 CM-CONNECTED 狀態
		require.False(t, ue.CmConnect(models.AccessType__3_GPP_ACCESS),
			"AmfUe should NOT be CM-CONNECTED after RanUe is detached")
	})
}

// TestAmfUeCmState_BothAccessTypes tests CM state with both access types connected
// 測試目標：驗證同時連接兩種 Access Type 時的狀態
func TestAmfUeCmState_BothAccessTypes(t *testing.T) {
	// 準備：建立 AmfUe 並初始化
	ue := &AmfUe{}
	ue.init()

	t.Run("Both Access Types Connected", func(t *testing.T) {
		// 連接 3GPP Access
		fakeRanUe3gpp := &RanUe{
			RanUeNgapId: 1,
			AmfUeNgapId: 1,
			Log:         logger.NgapLog.WithField("test", "3gpp"),
		}
		ue.RanUe[models.AccessType__3_GPP_ACCESS] = fakeRanUe3gpp

		// 連接 Non-3GPP Access
		fakeRanUeNon3gpp := &RanUe{
			RanUeNgapId: 2,
			AmfUeNgapId: 2,
			Log:         logger.NgapLog.WithField("test", "non3gpp"),
		}
		ue.RanUe[models.AccessType_NON_3_GPP_ACCESS] = fakeRanUeNon3gpp

		// 斷言：兩種 Access Type 都應該是 CM-CONNECTED
		require.True(t, ue.CmConnect(models.AccessType__3_GPP_ACCESS),
			"3GPP access should be CM-CONNECTED")
		require.True(t, ue.CmConnect(models.AccessType_NON_3_GPP_ACCESS),
			"Non-3GPP access should be CM-CONNECTED")

		// 斷言：兩種 Access Type 都不應該是 CM-IDLE
		require.False(t, ue.CmIdle(models.AccessType__3_GPP_ACCESS),
			"3GPP access should NOT be CM-IDLE")
		require.False(t, ue.CmIdle(models.AccessType_NON_3_GPP_ACCESS),
			"Non-3GPP access should NOT be CM-IDLE")
	})

	t.Run("Detach One Access Type", func(t *testing.T) {
		// 移除 3GPP Access
		delete(ue.RanUe, models.AccessType__3_GPP_ACCESS)

		// 斷言：3GPP Access 應該變回 CM-IDLE
		require.True(t, ue.CmIdle(models.AccessType__3_GPP_ACCESS),
			"3GPP access should return to CM-IDLE")

		// 斷言：Non-3GPP Access 應該仍然是 CM-CONNECTED
		require.True(t, ue.CmConnect(models.AccessType_NON_3_GPP_ACCESS),
			"Non-3GPP access should still be CM-CONNECTED")
	})
}

// TestAmfUeCmState_NilRanUe tests CM state when RanUe is explicitly set to nil
// 測試目標：驗證 RanUe 設為 nil 時的狀態處理
func TestAmfUeCmState_NilRanUe(t *testing.T) {
	// 準備：建立 AmfUe 並初始化
	ue := &AmfUe{}
	ue.init()

	t.Run("RanUe set to nil", func(t *testing.T) {
		// 明確設定 RanUe 為 nil (而不是刪除 key)
		ue.RanUe[models.AccessType__3_GPP_ACCESS] = nil

		// 斷言：應該被視為 CM-IDLE (因為 RanUe 是 nil)
		require.True(t, ue.CmIdle(models.AccessType__3_GPP_ACCESS),
			"AmfUe should be CM-IDLE when RanUe is nil")

		// 斷言：不應該是 CM-CONNECTED
		require.False(t, ue.CmConnect(models.AccessType__3_GPP_ACCESS),
			"AmfUe should NOT be CM-CONNECTED when RanUe is nil")
	})
}
