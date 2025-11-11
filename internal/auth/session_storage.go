package auth

import (
	"log"

	"github.com/gin-contrib/sessions"
	"github.com/logto-io/go/v2/client"
)

type SessionStorage struct {
	session sessions.Session
}

func NewSessionStorage(session sessions.Session) client.Storage {
	return &SessionStorage{session: session}
}

func (s *SessionStorage) GetItem(key string) string {
	value := s.session.Get(key)
	if value == nil {
		log.Printf("[SessionStorage] GetItem(%s) = <nil>", key)
		return ""
	}
	log.Printf("[SessionStorage] GetItem(%s) = %v", key, value)
	return value.(string)
}

func (s *SessionStorage) SetItem(key, value string) {
	log.Printf("[SessionStorage] SetItem(%s, %s)", key, value[:min(50, len(value))]+"...")
	s.session.Set(key, value)
	err := s.session.Save()
	if err != nil {
		log.Printf("[SessionStorage] ERROR saving session: %v", err)
	} else {
		log.Printf("[SessionStorage] Session saved successfully")
	}
}
