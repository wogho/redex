package server

import (
	"net/http"
	"os"
	"strings"

	"github.com/wogho/redex/config"
)

// GET /api/system/info — read-only runtime facts for the mode/policy page.
func (s *Server) systemInfo(w http.ResponseWriter, _ *http.Request) {
	policyPath := strings.TrimSpace(os.Getenv(config.PolicyFileEnv))
	policyExists := false
	if policyPath != "" {
		if _, err := os.Stat(policyPath); err == nil {
			policyExists = true
		}
	}
	layerPath := attackLayerPath()
	layerExists := false
	if _, err := os.Stat(layerPath); err == nil {
		layerExists = true
	}
	writeJSON(w, 200, map[string]any{
		"version":            BuildVersion,
		"mode":               map[bool]string{true: "offensive", false: "default"}[config.Offensive()],
		"offensive":          config.Offensive(),
		"require_human":      config.RequireHuman(),
		"language":           config.Language(),
		"policy_path":        policyPath,
		"policy_configured":  policyPath != "",
		"policy_exists":      policyExists,
		"attack_layer_path":  layerPath,
		"attack_layer_ready": layerExists,
	})
}
