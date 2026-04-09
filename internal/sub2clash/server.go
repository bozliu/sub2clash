package sub2clash

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

//go:embed web/admin.html
var adminPage string

type Server struct {
	cfg     *Config
	service *Service
	mux     *http.ServeMux
}

func NewServer(cfg *Config, service *Service) *Server {
	server := &Server{
		cfg:     cfg,
		service: service,
		mux:     http.NewServeMux(),
	}
	server.routes()
	return server
}

func (s *Server) routes() {
	s.mux.HandleFunc("/", s.handleAdminPage)
	s.mux.HandleFunc("/healthz", s.handleHealth)
	s.mux.HandleFunc("/api/profiles", s.withAdminAuth(s.handleProfiles))
	s.mux.HandleFunc("/api/profiles/", s.withAdminAuth(s.handleProfileRefresh))
	s.mux.HandleFunc("/profiles/", s.handleManagedProfile)
}

func (s *Server) Run(ctx context.Context) error {
	if err := s.cfg.ValidateForServe(); err != nil {
		return err
	}

	ticker := time.NewTicker(s.cfg.RefreshTicker)
	defer ticker.Stop()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.service.RefreshDueProfiles(context.Background()); err != nil {
					log.Printf("refresh loop: %v", err)
				}
			}
		}
	}()

	server := &http.Server{
		Addr:              s.cfg.Addr,
		Handler:           s.mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		_ = server.Shutdown(context.Background())
	}()

	log.Printf("sub2clash listening on %s", s.cfg.Addr)
	err := server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) handleAdminPage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(adminPage))
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "service": "sub2clash"})
}

func (s *Server) handleProfiles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := s.service.ListProfiles(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	case http.MethodPost:
		var req AdminCreateProfileRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid json body: %w", err))
			return
		}
		refreshInterval, err := ParseRefreshInterval(req.RefreshInterval)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		profile, err := s.service.AddProfile(r.Context(), req.Name, req.URL, refreshInterval)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, profile)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleProfileRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/profiles/")
	id, suffix, _ := strings.Cut(path, "/")
	if suffix != "refresh" || id == "" {
		http.NotFound(w, r)
		return
	}
	result, err := s.service.RefreshProfile(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, AdminRefreshResponse{
		ID:               id,
		Warnings:         result.Warnings,
		SubscriptionInfo: result.SubscriptionInfo,
	})
}

func (s *Server) handleManagedProfile(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/profiles/")
	id, suffix, _ := strings.Cut(path, "/")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	if suffix != "clash.yaml" && suffix != "download" {
		http.NotFound(w, r)
		return
	}
	body, userInfo, err := s.service.GetManagedYAML(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if userInfo != "" {
		w.Header().Set("subscription-userinfo", userInfo)
	}
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	if suffix == "download" {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.yaml"`, id))
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (s *Server) withAdminAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if token == "" {
			token = strings.TrimSpace(r.Header.Get("X-Admin-Token"))
		}
		if token == "" || token != s.cfg.AdminToken {
			writeError(w, http.StatusUnauthorized, errors.New("admin authorization required"))
			return
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{
		"error": err.Error(),
	})
}
