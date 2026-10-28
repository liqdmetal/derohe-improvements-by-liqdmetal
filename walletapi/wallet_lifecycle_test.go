package walletapi

import (
	"testing"
	"time"
)

// Simulator integration tests drive every sync explicitly and should not
// start a second asynchronous loop over the same wallet state.
func setWalletOnlineWithoutSyncLoop(wallet *Wallet_Disk) {
	if wallet == nil || wallet.Wallet_Memory == nil {
		return
	}
	memory := wallet.Wallet_Memory
	memory.syncLifecycleMu.Lock()
	memory.wallet_online_mode = true
	memory.syncLifecycleMu.Unlock()
}

func syncLoopDone(wallet *Wallet_Memory) <-chan struct{} {
	wallet.syncLifecycleMu.Lock()
	defer wallet.syncLifecycleMu.Unlock()
	return wallet.syncDone
}

func waitForSyncLoopExit(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("wallet sync loop did not stop within 2 seconds")
	}
}

func TestWalletSyncLoopLifecycle(t *testing.T) {
	closeSimulatorDaemonConnection()

	wallet, err := Create_Encrypted_Wallet_Random_Memory("sync-lifecycle-test")
	if err != nil {
		t.Fatalf("create in-memory wallet: %v", err)
	}
	t.Cleanup(func() {
		wallet.SetOfflineMode()
		if done := syncLoopDone(wallet); done != nil {
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Errorf("wallet sync loop leaked during test cleanup")
			}
		}
	})

	if wasOnline := wallet.SetOnlineMode(); wasOnline {
		t.Fatal("first SetOnlineMode call unexpectedly reported an existing online state")
	}
	firstDone := syncLoopDone(wallet)
	if firstDone == nil {
		t.Fatal("SetOnlineMode did not register a sync-loop completion channel")
	}
	if wasOnline := wallet.SetOnlineMode(); !wasOnline {
		t.Fatal("second SetOnlineMode call did not report the existing online state")
	}
	if got := syncLoopDone(wallet); got != firstDone {
		t.Fatal("repeated SetOnlineMode replaced the active sync loop")
	}

	if wasOnline := wallet.SetOfflineMode(); !wasOnline {
		t.Fatal("SetOfflineMode did not report the previous online state")
	}
	waitForSyncLoopExit(t, firstDone)
	if wallet.GetMode() {
		t.Fatal("wallet remained online after SetOfflineMode")
	}

	if wasOnline := wallet.SetOnlineMode(); wasOnline {
		t.Fatal("restart SetOnlineMode unexpectedly reported an existing online state")
	}
	secondDone := syncLoopDone(wallet)
	if secondDone == nil || secondDone == firstDone {
		t.Fatal("restarting online mode did not create a new sync-loop lifecycle")
	}
	if wasOnline := wallet.SetOfflineMode(); !wasOnline {
		t.Fatal("SetOfflineMode did not stop the restarted sync loop")
	}
	waitForSyncLoopExit(t, secondDone)
}
