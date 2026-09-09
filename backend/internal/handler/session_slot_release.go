package handler

import "github.com/Wei-Shaw/sub2api/internal/service"

type sessionSlotReleaseTracker struct {
	accounts map[int64]*service.Account
	retained bool
}

func newSessionSlotReleaseTracker() *sessionSlotReleaseTracker {
	return &sessionSlotReleaseTracker{accounts: make(map[int64]*service.Account)}
}

func (t *sessionSlotReleaseTracker) add(account *service.Account) {
	if account == nil {
		return
	}
	t.accounts[account.ID] = account
}

func (t *sessionSlotReleaseTracker) release(account *service.Account, release func(*service.Account)) {
	if account == nil || release == nil {
		return
	}
	release(account)
	delete(t.accounts, account.ID)
}

func (t *sessionSlotReleaseTracker) releaseAll(release func(*service.Account)) {
	if t.retained || release == nil {
		return
	}
	for id, account := range t.accounts {
		release(account)
		delete(t.accounts, id)
	}
}

func (t *sessionSlotReleaseTracker) retain() {
	t.retained = true
}
