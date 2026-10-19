package server

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/wogho/redex/config"
)

// attackLayerPath is where the generated ATT&CK Navigator layer lives in the
// image (see scripts/gen_offense_layer.py).
func attackLayerPath() string {
	if p := os.Getenv(config.AttackLayerEnv); p != "" {
		return p
	}
	return "/app/attack/redex_offense_layer.json"
}

// GET /api/attack/layer — returns the offense layer plus per-tactic counts.
func (s *Server) attackLayer(w http.ResponseWriter, _ *http.Request) {
	raw, err := os.ReadFile(attackLayerPath())
	if err != nil {
		writeErr(w, 500, "attack layer not available: "+err.Error())
		return
	}
	var layer struct {
		Name       string `json:"name"`
		Techniques []struct {
			TechniqueID string `json:"techniqueID"`
			Tactic      string `json:"tactic"`
			Comment     string `json:"comment"`
		} `json:"techniques"`
	}
	if err := json.Unmarshal(raw, &layer); err != nil {
		writeErr(w, 500, "attack layer parse: "+err.Error())
		return
	}
	counts := map[string]int{}
	for _, t := range layer.Techniques {
		counts[t.Tactic]++
	}
	writeJSON(w, 200, map[string]any{
		"name":       layer.Name,
		"total":      len(layer.Techniques),
		"counts":     counts,
		"techniques": layer.Techniques,
	})
}
