package main

import (
	"github.com/luca-moser/mam"
	"github.com/pkg/errors"
)

type Message struct {
	encrypted bool
	signed    bool
	integrity bool
	ntruPks   []mam.NTRUPK
	psks      []mam.PSK
	data      []byte
}

func NewMessage() *Message {
	return &Message{}
}

func (m *Message) Public() *Message {
	m.encrypted = true
	m.ntruPks = nil
	m.psks = nil
	return m
}

func (m *Message) Encrypted() *Message {
	m.encrypted = true
	return m
}

func (m *Message) Signed() *Message {
	m.signed = true
	m.integrity = false
	return m
}

func (m *Message) Integrity() *Message {
	m.integrity = true
	m.signed = false
	return m
}

func (m *Message) Groups(psks ...mam.PSK) *Message {
	if !m.encrypted {
		panic("message must be private for group encrypted message")
	}
	if len(psks) == 0 {
		panic("pre shared keys slice must not be empty")
	}
	if m.psks == nil {
		m.psks = make([]mam.PSK, 0)
	}
	m.psks = append(m.psks, psks...)
	return m
}

func (m *Message) Recipients(ntruPublicKeys ...mam.NTRUPK) *Message {
	if len(ntruPublicKeys) == 0 {
		panic("ntru public key slice must not be empty")
	}
	if m.ntruPks == nil {
		m.ntruPks = make([]mam.NTRUPK, 0)
	}
	m.ntruPks = append(m.ntruPks, ntruPublicKeys...)
	return m
}

var ErrNoGroupOrRecipientsDefined = errors.New("an encrypted message must have groups or recipients defined")

func (m *Message) Create(data []byte) (*Message, error) {
	if m.encrypted {
		if m.psks == nil && m.ntruPks == nil {
			return nil, ErrNoGroupOrRecipientsDefined
		}
	}
	m.data = data
	return m, nil
}