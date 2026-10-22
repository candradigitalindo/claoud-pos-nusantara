package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"
	"database/sql"
	"errors"

	"github.com/gofiber/fiber/v2"
)

func fnbError(c *fiber.Ctx, err error) error {
	var invalid services.ValidationError
	var conflict services.ConflictError
	switch {
	case errors.Is(err, services.ErrFnbScope):
		return c.Status(403).JSON(models.APIResponse{Error: err.Error()})
	case errors.Is(err, sql.ErrNoRows):
		return c.Status(404).JSON(models.APIResponse{Error: "Data F&B tidak ditemukan"})
	case errors.As(err, &invalid):
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	case errors.As(err, &conflict):
		return c.Status(409).JSON(models.APIResponse{Error: err.Error()})
	default:
		return c.Status(500).JSON(models.APIResponse{Error: "Gagal memproses data F&B"})
	}
}

func FnbOverview(c *fiber.Ctx) error {
	r, err := services.GetFnbOverview(c.Query("outlet_id"), getOutletScope(c))
	if err != nil {
		return fnbError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: r})
}

func FnbCosting(c *fiber.Ctx) error {
	r, err := services.GetFnbCosting(c.Query("outlet_id"), getOutletScope(c))
	if err != nil {
		return fnbError(c, err)
	}
	r = services.FilterFnbCosts(r, c.Query("menu_kind"), c.Query("search"))
	// Cost access alone does not grant the recipe composition.
	if !roleHasPermission(c, "recipes.view") {
		redactFnbComposition(r)
	}
	page, limit := getPagination(c)
	rows, total := pagePpicRows(r, page, limit)
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"rows": rows, "total": total, "page": page, "limit": limit}})
}

func GetFnbOutletPolicy(c *fiber.Ctx) error {
	r, err := services.GetFnbOutletPolicy(c.Params("id"), getOutletScope(c))
	if err != nil {
		return fnbError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: r})
}

func SaveFnbOutletPolicy(c *fiber.Ctx) error {
	var req models.FnbOutletPolicy
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	actor, _ := c.Locals("admin_username").(string)
	if err := services.SaveFnbOutletPolicy(c.Params("id"), req, getOutletScope(c), actor); err != nil {
		return fnbError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true})
}

func SaveFnbProfile(c *fiber.Ctx) error {
	var req models.FnbProfile
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	actor, _ := c.Locals("admin_username").(string)
	if err := services.SaveFnbProfile(c.Params("id"), req, getOutletScope(c), actor); err != nil {
		return fnbError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true})
}

func ListFnbDevelopments(c *fiber.Ctx) error {
	r, err := services.ListFnbDevelopments(c.Query("outlet_id"), getOutletScope(c))
	if err != nil {
		return fnbError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: r})
}

func SaveFnbDevelopment(c *fiber.Ctx) error {
	var req models.FnbDevelopment
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	actor, _ := c.Locals("admin_username").(string)
	id, err := services.SaveFnbDevelopment(c.Params("id"), req, getOutletScope(c), actor)
	if err != nil {
		return fnbError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"id": id}})
}

func TransitionFnbDevelopment(c *fiber.Ctx) error {
	var req struct {
		Status  string `json:"status"`
		Version int    `json:"version"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	perm := "fnb.development.update"
	if req.Status == "approved" {
		perm = "fnb.development.approve"
	}
	if !roleHasPermission(c, perm) {
		return c.Status(403).JSON(models.APIResponse{Error: "Anda tidak memiliki izin untuk tahap ini"})
	}
	actor, _ := c.Locals("admin_username").(string)
	if err := services.TransitionFnbDevelopment(c.Params("id"), req.Status, req.Version, getOutletScope(c), actor, req.Status == "approved"); err != nil {
		return fnbError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true})
}

func FnbDevelopmentOptions(c *fiber.Ctx) error {
	outlet := c.Query("outlet_id")
	if outlet == "" {
		return c.Status(400).JSON(models.APIResponse{Error: "Pilih outlet terlebih dahulu"})
	}
	if !validateOutletAccess(c, outlet) {
		return c.Status(403).JSON(models.APIResponse{Error: "Outlet di luar akses Anda"})
	}
	recipes, err := services.ListProductRecipeOptions(outlet)
	if err != nil {
		return fnbError(c, err)
	}
	products, _, err := services.GetAllProducts(outlet, "", getOutletScope(c), 1, 10000)
	if err != nil {
		return fnbError(c, err)
	}
	options := []models.ProductRecipeOption{}
	for _, p := range products {
		if !p.IsDeleted {
			options = append(options, models.ProductRecipeOption{ID: p.ID, Name: p.Name})
		}
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"recipes": recipes, "products": options}})
}

func redactFnbComposition(r []models.FnbCostRow) {
	for i := range r {
		r[i].Ingredients = nil
		r[i].PriceComparisons = nil
		for j := range r[i].Recipes {
			r[i].Recipes[j].CostIngredients = nil
			if r[i].Recipes[j].InitialCost != nil {
				snapshot := *r[i].Recipes[j].InitialCost
				snapshot.Ingredients = nil
				r[i].Recipes[j].InitialCost = &snapshot
			}
		}
	}
}
