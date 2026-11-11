package server

import (
	"net/http"

	"github.com/abjelosevic88/pocket-homelab-math/internal/notify"
)

const notifyTestBody = "This is how Pocket Math tells you the day's practice is not done yet."

func (s *Server) handleNotifyTest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	st := s.settingsOf(ctx, actorID(ctx))
	if st.NotifyURL == "" {
		writeErr(w, http.StatusBadRequest, "add a notification URL first")
		return
	}
	n := notify.New(st.NotifyURL, st.NotifyFormat)
	if err := n.Send(ctx, notify.Message{Title: "Pocket Math", Body: notifyTestBody, Priority: "info", Tags: []string{"abacus"}}); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
