package server

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Credential vault: secrets encrypted with AES-GCM using a key derived from the
// server JWT key. The API never returns ciphertext or plaintext in list calls;
// reveal is an explicit, audited action.

func (s *Server) vaultKey() [32]byte { return sha256.Sum256(s.jwtKey) }

func (s *Server) encryptSecret(plain string) (string, error) {
	key := s.vaultKey()
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

func (s *Server) decryptSecret(enc string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}
	key := s.vaultKey()
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ns := gcm.NonceSize()
	if len(raw) < ns {
		return "", fmt.Errorf("bad ciphertext")
	}
	plain, err := gcm.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

type credentialView struct {
	ID        int64     `json:"id,string"`
	Name      string    `json:"name"`
	Kind      string    `json:"kind"`
	Service   string    `json:"service"`
	Username  string    `json:"username"`
	Hint      string    `json:"hint"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
}

// GET /api/credentials
func (s *Server) credentialsList(w http.ResponseWriter, r *http.Request) {
	rows, err := s.m.pg.QueryContext(r.Context(), `
SELECT id, name, kind, service, username, hint, notes, created_at
  FROM credential_entry ORDER BY created_at DESC`)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	out := make([]credentialView, 0, 16)
	for rows.Next() {
		var v credentialView
		if err := rows.Scan(&v.ID, &v.Name, &v.Kind, &v.Service, &v.Username, &v.Hint, &v.Notes, &v.CreatedAt); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		out = append(out, v)
	}
	writeJSON(w, 200, map[string]any{"credentials": out})
}

// POST /api/credentials
func (s *Server) credentialsCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string `json:"name"`
		Kind     string `json:"kind"`
		Service  string `json:"service"`
		Username string `json:"username"`
		Secret   string `json:"secret"`
		Notes    string `json:"notes"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if strings.TrimSpace(body.Name) == "" || strings.TrimSpace(body.Secret) == "" {
		writeErr(w, 400, "name and secret are required")
		return
	}
	enc, err := s.encryptSecret(body.Secret)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	hint := "••••"
	if len(body.Secret) >= 4 {
		hint = "••••" + body.Secret[len(body.Secret)-2:]
	}
	var id int64
	if err := s.m.pg.QueryRowContext(r.Context(), `
INSERT INTO credential_entry(name,kind,service,username,secret_enc,hint,notes)
VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		body.Name, body.Kind, body.Service, body.Username, enc, hint, body.Notes).Scan(&id); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "id": id})
}

// POST /api/credentials/{id}/reveal — explicit, audited
func (s *Server) credentialsReveal(w http.ResponseWriter, r *http.Request) {
	id, err := parseInt64(r.PathValue("id"))
	if err != nil || id <= 0 {
		writeErr(w, 400, "bad id")
		return
	}
	var enc string
	if err := s.m.pg.QueryRowContext(r.Context(), `SELECT secret_enc FROM credential_entry WHERE id=$1`, id).Scan(&enc); err != nil {
		writeErr(w, 404, "credential not found")
		return
	}
	plain, err := s.decryptSecret(enc)
	if err != nil {
		writeErr(w, 500, "decrypt failed")
		return
	}
	writeJSON(w, 200, map[string]any{"id": fmt.Sprintf("%d", id), "secret": plain})
}

// DELETE /api/credentials/{id}
func (s *Server) credentialsDelete(w http.ResponseWriter, r *http.Request) {
	id, err := parseInt64(r.PathValue("id"))
	if err != nil || id <= 0 {
		writeErr(w, 400, "bad id")
		return
	}
	_, _ = s.m.pg.ExecContext(r.Context(), `DELETE FROM credential_entry WHERE id=$1`, id)
	writeJSON(w, 200, map[string]any{"ok": true})
}
