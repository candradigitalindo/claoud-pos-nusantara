package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"database/sql"
	"strings"

	"github.com/lib/pq"
)

func normalizeProductRecipes(tx *sql.Tx, outlet string, requested *[]models.ProductRecipeLink, legacy string) ([]models.ProductRecipeLink, error) {
	links := []models.ProductRecipeLink{}
	if requested != nil {
		links = append(links, (*requested)...)
	} else if strings.TrimSpace(legacy) != "" {
		links = append(links, models.ProductRecipeLink{RecipeMasterID: legacy, Multiplier: 1})
	}
	seen := map[string]bool{}
	for i := range links {
		l := &links[i]
		l.RecipeMasterID = strings.TrimSpace(l.RecipeMasterID)
		if l.RecipeMasterID == "" || seen[l.RecipeMasterID] || !fnbFinite(l.Multiplier) || l.Multiplier <= 0 || l.Multiplier > 99999999999 || round4(l.Multiplier) <= 0 {
			return nil, ErrProductRecipe
		}
		seen[l.RecipeMasterID] = true
		name, err := validateProductRecipe(tx, outlet, l.RecipeMasterID)
		if err != nil {
			return nil, err
		}
		l.Name, l.Multiplier = name, round4(l.Multiplier)
	}
	return links, nil
}

func replaceProductRecipes(tx *sql.Tx, productID string, links []models.ProductRecipeLink) error {
	if _, err := tx.Exec(`DELETE FROM product_recipe_links WHERE product_id=$1`, productID); err != nil {
		return err
	}
	for i, l := range links {
		if _, err := tx.Exec(`INSERT INTO product_recipe_links(product_id,recipe_master_id,multiplier,position) VALUES($1,$2,$3,$4)`, productID, l.RecipeMasterID, l.Multiplier, i); err != nil {
			return err
		}
	}
	return nil
}

func loadProductRecipeLinks(ids []string) (map[string][]models.ProductRecipeLink, error) {
	result := map[string][]models.ProductRecipeLink{}
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := database.DB.Query(`SELECT s.product_id,s.recipe_master_id,r.name,s.multiplier FROM product_recipe_sources s
		JOIN recipe_masters r ON r.id=s.recipe_master_id WHERE s.product_id=ANY($1) ORDER BY s.position,r.name,r.id`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var pid string
		var l models.ProductRecipeLink
		if err := rows.Scan(&pid, &l.RecipeMasterID, &l.Name, &l.Multiplier); err != nil {
			return nil, err
		}
		result[pid] = append(result[pid], l)
	}
	return result, rows.Err()
}

func hydrateProductRecipes(products []models.CloudProduct) error {
	ids := make([]string, 0, len(products))
	for _, p := range products {
		ids = append(ids, p.ID)
	}
	links, err := loadProductRecipeLinks(ids)
	if err != nil {
		return err
	}
	for i := range products {
		products[i].Recipes = links[products[i].ID]
		if len(products[i].Recipes) > 0 {
			products[i].RecipeMasterID = products[i].Recipes[0].RecipeMasterID
			products[i].RecipeMasterName = products[i].Recipes[0].Name
		}
		if products[i].Recipes == nil {
			products[i].Recipes = []models.ProductRecipeLink{}
		}
	}
	return nil
}
