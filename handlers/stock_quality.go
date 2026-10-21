package handlers

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"cloud-pos/services"
	"database/sql"
	"errors"
	"io"

	"github.com/gofiber/fiber/v2"
	"github.com/lib/pq"
)

func stockQualityAccess(c *fiber.Ctx) (services.StockQualityAccess, error) {
	a := services.StockQualityAccess{Scope: getOutletScope(c), Permissions: map[string]bool{}}
	a.ActorID, _ = c.Locals("admin_id").(string)
	a.ActorName, _ = c.Locals("admin_username").(string)
	role, _ := c.Locals("admin_role").(string)
	if role == "superadmin" {
		a.Permissions["*"] = true
		return a, nil
	}
	perms, err := services.GetRolePermissions(role)
	if err != nil {
		return a, err
	}
	for _, p := range perms {
		a.Permissions[p] = true
	}
	return a, nil
}

func stockQualityError(c *fiber.Ctx, err error) error {
	var invalid services.ValidationError
	var conflict services.ConflictError
	switch {
	case errors.Is(err, services.ErrStockQualityAccess):
		return c.Status(403).JSON(models.APIResponse{Error: err.Error()})
	case errors.Is(err, sql.ErrNoRows):
		return c.Status(404).JSON(models.APIResponse{Error: "Dokumen/gudang tidak ditemukan"})
	case errors.As(err, &invalid):
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	case errors.As(err, &conflict):
		return c.Status(409).JSON(models.APIResponse{Error: err.Error()})
	default:
		return c.Status(500).JSON(models.APIResponse{Error: "Gagal memproses dokumen mutu stok"})
	}
}

func UploadStockEvidence(c *fiber.Ctx) error {
	a, err := stockQualityAccess(c)
	if err != nil {
		return stockQualityError(c, err)
	}
	f, err := c.FormFile("file")
	if err != nil || f.Size > 5*1024*1024 {
		return c.Status(400).JSON(models.APIResponse{Error: "Foto wajib diunggah, maksimal 5 MB"})
	}
	file, err := f.Open()
	if err != nil {
		return stockQualityError(c, err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 5*1024*1024+1))
	if err != nil {
		return stockQualityError(c, err)
	}
	p, err := services.CreateStockEvidence(c.FormValue("warehouse_id"), data, a)
	if err != nil {
		return stockQualityError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: p})
}
func ListStockQuality(c *fiber.Ctx) error {
	a, err := stockQualityAccess(c)
	if err != nil {
		return stockQualityError(c, err)
	}
	page, limit := getPagination(c)
	rows, total, err := services.ListStockQuality(c.Query("warehouse_id"), c.Query("status"), c.Query("search"), page, limit, a)
	if err != nil {
		return stockQualityError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"rows": rows, "total": total, "page": page, "limit": limit}})
}
func GetStockQuality(c *fiber.Ctx) error {
	a, err := stockQualityAccess(c)
	if err != nil {
		return stockQualityError(c, err)
	}
	doc, err := services.GetStockQuality(c.Params("id"), a)
	if err != nil {
		return stockQualityError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: doc})
}
func CreateStockQuality(c *fiber.Ctx) error {
	a, err := stockQualityAccess(c)
	if err != nil {
		return stockQualityError(c, err)
	}
	var req models.StockQualityRequest
	if err = c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "Data tidak valid"})
	}
	id, err := services.CreateStockQuality(req, a)
	if err != nil {
		return stockQualityError(c, err)
	}
	return c.Status(201).JSON(models.APIResponse{Success: true, Data: fiber.Map{"id": id}})
}
func ActStockQuality(c *fiber.Ctx) error {
	a, err := stockQualityAccess(c)
	if err != nil {
		return stockQualityError(c, err)
	}
	var req models.StockQualityAction
	if err = c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "Data tidak valid"})
	}
	if err = services.ActStockQuality(c.Params("id"), req, a); err != nil {
		return stockQualityError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true})
}
func ReceiveStockTransfer(c *fiber.Ctx) error {
	a, err := stockQualityAccess(c)
	if err != nil {
		return stockQualityError(c, err)
	}
	var req models.StockInspectionRequest
	if err = c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "Data tidak valid"})
	}
	if err = services.ReceiveStockTransfer(c.Params("id"), req, a); err != nil {
		return stockQualityError(c, err)
	}
	doc, err := services.GetStockTransfer(c.Params("id"))
	if err != nil {
		return stockQualityError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: doc})
}
func StockQualityBatches(c *fiber.Ctx) error {
	a, err := stockQualityAccess(c)
	if err != nil {
		return stockQualityError(c, err)
	}
	rows, err := services.StockQualityBatches(c.Query("warehouse_id"), c.Query("item_id"), a)
	if err != nil {
		return stockQualityError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: rows})
}

func StockQualityLookups(c *fiber.Ctx) error {
	a, err := stockQualityAccess(c)
	if err != nil {
		return stockQualityError(c, err)
	}
	rows, err := database.DB.Query(`SELECT id,name,type FROM warehouses WHERE is_active AND ($1 OR type='outlet' AND outlet_id=ANY($2)) ORDER BY name`, a.Scope == nil, pq.Array(a.Scope))
	if err != nil {
		return stockQualityError(c, err)
	}
	warehouses := []fiber.Map{}
	for rows.Next() {
		var id, name, kind string
		if err = rows.Scan(&id, &name, &kind); err != nil {
			rows.Close()
			return stockQualityError(c, err)
		}
		warehouses = append(warehouses, fiber.Map{"id": id, "name": name, "type": kind})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return stockQualityError(c, err)
	}
	names := func(query string) ([]fiber.Map, error) {
		r, err := database.DB.Query(query)
		if err != nil {
			return nil, err
		}
		defer r.Close()
		result := []fiber.Map{}
		for r.Next() {
			var id, name string
			if err := r.Scan(&id, &name); err != nil {
				return nil, err
			}
			result = append(result, fiber.Map{"id": id, "name": name})
		}
		return result, r.Err()
	}
	central, err := names(`SELECT id,name FROM warehouses WHERE type='central' AND is_active ORDER BY name`)
	if err != nil {
		return stockQualityError(c, err)
	}
	vendors := []fiber.Map{}
	if a.Scope == nil {
		vendors, err = names(`SELECT id,name FROM vendors WHERE is_active ORDER BY name`)
		if err != nil {
			return stockQualityError(c, err)
		}
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"warehouses": warehouses, "return_warehouses": central, "vendors": vendors}})
}

func LegacyStockWasteDisabled(c *fiber.Ctx) error {
	return c.Status(410).JSON(models.APIResponse{Error: "Laporkan barang melalui menu Reject, Retur & Kedaluwarsa dengan foto bertimestamp dan persetujuan supervisor"})
}
