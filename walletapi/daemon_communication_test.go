package walletapi

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/server"
	"github.com/gorilla/websocket"
)

type blockingRPCAssigner struct {
	started chan struct{}
	once    sync.Once
}

func (a *blockingRPCAssigner) Assign(context.Context, string) jrpc2.Handler {
	return blockingRPCHandler{assigner: a}
}

type blockingRPCHandler struct {
	assigner *blockingRPCAssigner
}

func (h blockingRPCHandler) Handle(ctx context.Context, _ *jrpc2.Request) (interface{}, error) {
	h.assigner.once.Do(func() { close(h.assigner.started) })
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestClientCallContextReturnsWhenCancelled(t *testing.T) {
	assigner := &blockingRPCAssigner{started: make(chan struct{})}
	local := server.NewLocal(assigner, nil)
	defer local.Close()

	client := &Client{RPC: local.Client}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	callDone := make(chan error, 1)
	go func() {
		var result struct{}
		callDone <- client.CallContext(ctx, "test.Blocking", nil, &result)
	}()

	select {
	case <-assigner.started:
	case <-time.After(time.Second):
		t.Fatal("RPC handler did not start")
	}
	cancel()

	select {
	case err := <-callDone:
		if err == nil {
			t.Fatal("canceled RPC unexpectedly succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("CallContext did not return after cancellation")
	}
}

func TestSetOfflineModeStopsSyncLoopDuringRPCCall(t *testing.T) {
	assigner := &blockingRPCAssigner{started: make(chan struct{})}
	local := server.NewLocal(assigner, nil)
	t.Cleanup(func() { _ = local.Close() })

	previousClient := rpc_client
	rpc_client = &Client{WS: &websocket.Conn{}, RPC: local.Client}
	t.Cleanup(func() { rpc_client = previousClient })

	wallet, err := Create_Encrypted_Wallet_Random_Memory("sync-cancel-test")
	if err != nil {
		t.Fatalf("create in-memory wallet: %v", err)
	}
	t.Cleanup(func() {
		wallet.SetOfflineMode()
		if done := syncLoopDone(wallet); done != nil {
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Errorf("sync loop leaked during cancellation test cleanup")
			}
		}
	})

	if wallet.SetOnlineMode() {
		t.Fatal("SetOnlineMode reported an existing sync loop")
	}
	done := syncLoopDone(wallet)
	if done == nil {
		t.Fatal("SetOnlineMode did not create a sync-loop completion channel")
	}

	select {
	case <-assigner.started:
	case <-time.After(time.Second):
		t.Fatal("sync loop did not start its daemon RPC")
	}
	if !wallet.SetOfflineMode() {
		t.Fatal("SetOfflineMode did not report the active sync loop")
	}
	waitForSyncLoopExit(t, done)

}
