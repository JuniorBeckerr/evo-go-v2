package limits

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type fakeMex struct {
	data    string
	err     error
	queryID string
	vars    any
}

func (f *fakeMex) SendMexIQ(_ context.Context, queryID string, variables any) (json.RawMessage, error) {
	f.queryID, f.vars = queryID, variables
	if f.data == "" {
		return nil, f.err
	}
	return json.RawMessage(f.data), f.err
}

func TestFetchNewChatMessageCappingInfo(t *testing.T) {
	f := &fakeMex{data: `{"xwa2_message_capping_info":{"total_quota":50,"used_quota":12,"cycle_end_timestamp":"1760000000","capping_status":"FIRST_WARNING","ote_status":"ELIGIBLE","mv_status":"NOT_ACTIVE"}}`}
	info, err := FetchNewChatMessageCappingInfo(context.Background(), f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.queryID != QueryNewChatMessageCappingInfo {
		t.Fatalf("wrong query id %q", f.queryID)
	}
	in := f.vars.(map[string]any)["input"].(map[string]any)
	if in["type"] != "INDIVIDUAL_NEW_CHAT_MSG" {
		t.Fatalf("wrong variables %v", f.vars)
	}
	if info.TotalQuota != 50 || info.UsedQuota != 12 || info.CappingStatus != NewChatMessageCappingStatusFirstWarning {
		t.Fatalf("bad parse: %+v", info)
	}
	if info.CycleEndTimestamp.Unix() != 1760000000 {
		t.Fatalf("bad cycle end: %v", info.CycleEndTimestamp.Unix())
	}
}

func TestFetchNewChatMessageCappingInfoNull(t *testing.T) {
	_, err := FetchNewChatMessageCappingInfo(context.Background(), &fakeMex{data: `{"xwa2_message_capping_info":null}`})
	if err == nil {
		t.Fatal("expected error on null response")
	}
}

func TestFetchAccountReachoutTimelock(t *testing.T) {
	f := &fakeMex{data: `{"xwa2_fetch_account_reachout_timelock":{"is_active":true,"time_enforcement_ends":"1760003600","enforcement_type":"BIZ_QUALITY"}}`}
	tl, err := FetchAccountReachoutTimelock(context.Background(), f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.queryID != QueryAccountReachoutTimelock {
		t.Fatalf("wrong query id %q", f.queryID)
	}
	if !tl.IsActive || tl.EnforcementType != ReachoutTimelockEnforcementTypeBizQuality || tl.TimeEnforcementEnds.Unix() != 1760003600 {
		t.Fatalf("bad parse: %+v", tl)
	}
}

func TestFetchAccountReachoutTimelockError(t *testing.T) {
	want := errors.New("boom")
	_, err := FetchAccountReachoutTimelock(context.Background(), &fakeMex{err: want})
	if !errors.Is(err, want) {
		t.Fatalf("expected %v, got %v", want, err)
	}
}

func TestNilClient(t *testing.T) {
	if _, err := GetNewChatMessageCappingInfo(context.Background(), nil); err == nil {
		t.Fatal("expected error for nil client")
	}
	if _, err := GetAccountReachoutTimelock(context.Background(), nil); err == nil {
		t.Fatal("expected error for nil client")
	}
}
