package handlers

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"cloud-pos/services"
	"database/sql"
	"errors"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/lib/pq"
)

func financeAccess(c *fiber.Ctx) (services.FinanceAccess, error) {
	a := services.FinanceAccess{Outlets: getOutletScope(c), WorkUnits: getWorkUnitScope(c), Permissions: map[string]bool{}}
	a.ActorID, _ = c.Locals("admin_id").(string)
	a.Actor, _ = c.Locals("admin_username").(string)
	role, _ := c.Locals("admin_role").(string)
	if role == "superadmin" {
		a.Permissions["*"] = true
		return a, nil
	}
	perms, err := services.GetRolePermissions(role)
	for _, p := range perms {
		a.Permissions[p] = true
	}
	return a, err
}
func financeError(c *fiber.Ctx, err error) error {
	var invalid services.ValidationError
	var conflict services.ConflictError
	status, message := 500, "Gagal memproses keuangan"
	switch {
	case errors.Is(err, services.ErrFinanceAccess):
		status, message = 403, err.Error()
	case errors.Is(err, sql.ErrNoRows):
		status, message = 404, "Transaksi tidak ditemukan"
	case errors.As(err, &invalid):
		status, message = 400, err.Error()
	case errors.As(err, &conflict):
		status, message = 409, err.Error()
	default:
		log.Printf("finance: %v", err)
	}
	return c.Status(status).JSON(models.APIResponse{Error: message})
}
func FinanceAccounts(c *fiber.Ctx) error {
	return c.JSON(models.APIResponse{Success: true, Data: services.FinanceAccounts})
}
func FinanceLookups(c *fiber.Ctx) error {
	a, err := financeAccess(c)
	if err != nil {
		return financeError(c, err)
	}
	q := `SELECT id,name FROM outlets WHERE is_active=true`
	args := []interface{}{}
	if a.Outlets != nil {
		q += ` AND id=ANY($1::text[])`
		args = append(args, pq.Array(a.Outlets))
	}
	q += ` ORDER BY name`
	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return financeError(c, err)
	}
	outlets := []fiber.Map{}
	for rows.Next() {
		var id, name string
		if err = rows.Scan(&id, &name); err != nil {
			rows.Close()
			return financeError(c, err)
		}
		outlets = append(outlets, fiber.Map{"id": id, "name": name})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return financeError(c, err)
	}
	units, err := services.ListWorkUnits(a.WorkUnits)
	if err != nil {
		return financeError(c, err)
	}
	banks, err := services.ListBankAccounts()
	if err != nil {
		return financeError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"accounts": services.FinanceAccounts, "outlets": outlets, "work_units": units, "banks": banks, "allow_central": a.Outlets == nil}})
}
func ListFinanceOperations(c *fiber.Ctx) error {
	from, to, err := getDateRange(c)
	if err != nil {
		return financeError(c, services.Invalid("%s", err))
	}
	if id := c.Query("outlet_id"); id != "" && !validateOutletAccess(c, id) {
		return financeError(c, services.ErrFinanceAccess)
	}
	page, limit := getPagination(c)
	data, err := services.ListFinanceOperations(from, to, c.Query("outlet_id"), c.Query("source"), c.Query("direction"), c.Query("account"), c.Query("search"), page, limit, getOutletScope(c))
	if err != nil {
		return financeError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: data})
}
func ListFinanceEntries(c *fiber.Ctx) error {
	from, to, err := getDateRange(c)
	if err != nil {
		return financeError(c, services.Invalid("%s", err))
	}
	a, err := financeAccess(c)
	if err != nil {
		return financeError(c, err)
	}
	data, err := services.ListFinanceEntries(from, to, c.Query("outlet_id"), a)
	if err != nil {
		return financeError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: data})
}
func GetFinanceEntry(c *fiber.Ctx) error {
	a, err := financeAccess(c)
	if err != nil {
		return financeError(c, err)
	}
	data, err := services.GetFinanceEntry(c.Params("id"), a)
	if err != nil {
		return financeError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: data})
}
func SaveFinanceEntry(c *fiber.Ctx) error {
	a, err := financeAccess(c)
	if err != nil {
		return financeError(c, err)
	}
	var req models.FinanceEntryInput
	if err = c.BodyParser(&req); err != nil {
		return financeError(c, services.Invalid("data transaksi tidak valid"))
	}
	var data *models.FinanceEntry
	if c.Params("id") == "" {
		data, err = services.CreateFinanceEntry(req, a)
	} else {
		data, err = services.UpdateFinanceEntry(c.Params("id"), req, a)
	}
	if err != nil {
		return financeError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: data})
}
func ClassifyFinanceCash(c *fiber.Ctx) error {
	a, err := financeAccess(c)
	if err != nil {
		return financeError(c, err)
	}
	var req struct {
		Account string `json:"account"`
		Notes   string `json:"notes"`
	}
	if err = c.BodyParser(&req); err != nil {
		return financeError(c, services.Invalid("data klasifikasi tidak valid"))
	}
	if err = services.ClassifyFinanceCash(c.Params("id"), req.Account, req.Notes, a); err != nil {
		return financeError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true})
}
func ActFinanceEntry(c *fiber.Ctx) error {
	a, err := financeAccess(c)
	if err != nil {
		return financeError(c, err)
	}
	var req struct {
		Action  string `json:"action"`
		Reason  string `json:"reason"`
		Version int    `json:"version"`
	}
	if err = c.BodyParser(&req); err != nil {
		return financeError(c, services.Invalid("data tindakan tidak valid"))
	}
	data, err := services.ActFinanceEntry(c.Params("id"), req.Action, req.Reason, req.Version, a)
	if err != nil {
		return financeError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: data})
}
