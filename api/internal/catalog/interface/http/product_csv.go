package httpapi

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/rifqif16/posq/api/internal/catalog/application"
	"github.com/rifqif16/posq/api/internal/platform/httpx"
)

type ProductCSVHandler struct {
	svc *application.ProductCSVService
	log *slog.Logger
}

func NewProductCSVHandler(svc *application.ProductCSVService, log *slog.Logger) *ProductCSVHandler {
	return &ProductCSVHandler{svc: svc, log: log}
}

func (h *ProductCSVHandler) Mount(r chi.Router, require Guard) {
	r.With(require(permProductCostRead)).Get("/products/export", h.export)
	r.With(require(permProductWrite)).Post("/products/import", h.importCSV)
}

type rowErrorDTO struct {
	Row     int    `json:"row"`
	Column  string `json:"column"`
	Message string `json:"message"`
}

type importReportDTO struct {
	Valid      bool          `json:"valid"`
	Products   int           `json:"products"`
	Variants   int           `json:"variants"`
	Created    int           `json:"created"`
	ErrorCount int           `json:"error_count"`
	Errors     []rowErrorDTO `json:"errors"`
}

type invalidImportDTO struct {
	Code   string `json:"code"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail"`
	importReportDTO
}

func toReportDTO(rep application.ImportReport) importReportDTO {
	errs := make([]rowErrorDTO, len(rep.Errors))
	for i, e := range rep.Errors {
		errs[i] = rowErrorDTO{Row: e.Row, Column: e.Column, Message: e.Message}
	}
	return importReportDTO{
		Valid: rep.Valid, Products: rep.Products, Variants: rep.Variants, Created: rep.Created,
		ErrorCount: rep.ErrorCount, Errors: errs,
	}
}

func (h *ProductCSVHandler) export(w http.ResponseWriter, r *http.Request) {
	delimiter := ','
	switch r.URL.Query().Get("delimiter") {
	case "", "comma":
	case "semicolon":
		delimiter = ';'
	default:
		httpx.WriteProblem(w, r, http.StatusBadRequest, "BAD_REQUEST", "delimiter harus comma atau semicolon")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="produk-`+time.Now().Format("20060102")+`.csv"`)
	if err := h.svc.Export(r.Context(), actor(r), w, delimiter); err != nil {
		h.log.Error("export produk gagal", "err", err)
	}
}

func (h *ProductCSVHandler) importCSV(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, application.MaxCSVBytes))
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "Ukuran file maksimal 2 MB")
		return
	}
	commit := r.URL.Query().Get("commit") == "true"
	rep, err := h.svc.Import(r.Context(), actor(r), data, commit)
	switch {
	case errors.Is(err, application.ErrSKUTaken):
		httpx.WriteProblem(w, r, http.StatusConflict, "SKU_TAKEN", "SKU sudah dipakai produk lain, validasi ulang file")
	case errors.Is(err, application.ErrBarcodeTaken):
		httpx.WriteProblem(w, r, http.StatusConflict, "BARCODE_TAKEN", "Barcode sudah dipakai produk lain, validasi ulang file")
	case errors.Is(err, application.ErrInvalidCategory):
		httpx.WriteProblem(w, r, http.StatusConflict, "INVALID_CATEGORY", "Kategori berubah saat impor, validasi ulang file")
	case err != nil:
		h.log.Error("impor produk gagal", "err", err)
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "INTERNAL", "Terjadi kesalahan pada server")
	case commit && !rep.Valid:
		httpx.WriteJSON(w, http.StatusUnprocessableEntity, invalidImportDTO{
			Code: "CSV_INVALID", Title: http.StatusText(http.StatusUnprocessableEntity), Status: http.StatusUnprocessableEntity,
			Detail: "File CSV mengandung kesalahan; tidak ada produk yang dibuat", importReportDTO: toReportDTO(rep),
		})
	case commit:
		httpx.WriteJSON(w, http.StatusCreated, toReportDTO(rep))
	default:
		httpx.WriteJSON(w, http.StatusOK, toReportDTO(rep))
	}
}
