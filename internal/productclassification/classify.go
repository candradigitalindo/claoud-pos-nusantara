// Package productclassification provides a conservative, deterministic menu
// classifier. Existing labels are authoritative; ambiguous evidence is never
// converted into an automatic activity label.
package productclassification

import (
	"cloud-pos/models"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"unicode"
)

const rulesVersion = "menu-kind-v2-atv-overland"

var nameRules = map[string][]string{
	"food": {
		"nasi", "rice", "mie", "mi", "noodle", "noodles", "bakmi", "kwetiau", "bihun",
		"ayam", "chicken", "bebek", "duck", "sate", "satay", "soto", "bakso", "sop", "sup", "soup",
		"iga", "steak", "burger", "pizza", "pasta", "spaghetti", "sandwich", "toast", "roti", "bread",
		"cake", "waffle", "pancake", "crepe", "cookies", "cookie", "brownies", "brownie", "pudding", "puding",
		"pisang", "singkong", "tempe", "tahu", "kentang", "fries", "omelette", "omelet", "nugget",
		"siomay", "batagor", "dimsum", "dim sum", "donat", "donut", "salad", "udang", "shrimp",
		"cumi", "squid", "ikan", "fish", "gurame", "gurami", "lele", "rendang", "tongseng", "gulai",
		"geprek", "pecel", "gado gado", "ketoprak", "pempek", "seblak", "cilok", "cireng", "risol",
		"martabak", "sosis", "sausage", "churros", "croissant", "ice cream", "icecream", "es krim", "eskrim",
		"panna cotta", "pannacotta", "pannacota", "sushi", "sashimi", "ramen", "bento", "ricebowl", "food", "snack",
		"donuts", "eclair", "dessert", "tortilla", "chips", "aglio olio", "carbonara", "belut", "dori", "koloke", "laksa",
		"toppoki", "samyang", "pepperoni", "bakwan", "prawn", "jagung", "nachos", "crepes", "sambel", "sambal", "buffet",
		"chip", "dango", "pie", "marshmello", "marshmallow",
	},
	"beverage": {
		"kopi", "coffee", "espresso", "americano", "cappuccino", "latte", "macchiato", "mocha", "moccacino",
		"teh", "tea", "jus", "juice", "soda", "lemonade", "mocktail", "cocktail", "smoothie", "smoothies",
		"milkshake", "milk shake", "susu", "milk", "yakult", "matcha", "wedang", "bandrek", "bajigur",
		"air mineral", "mineral water", "coconut water", "kelapa muda", "hot chocolate", "coklat panas",
		"soft drink", "softdrink", "drink", "beer", "bir", "soju", "coffe", "mochaccino",
		"espreso", "sanger", "milky", "squash", "mojito", "air hangat", "air putih", "es batu", "es kosong", "iced chocolate",
	},
	"activity": {
		"billiard", "billiards", "biliar", "bilyard", "pool table", "karaoke", "playstation", "ps3", "ps4", "ps5",
		"atv", "paintball", "archery", "panahan", "flying fox", "rafting", "arung jeram", "horse riding", "berkuda",
		"go kart", "gokart", "tiket masuk", "entrance ticket", "day pass", "kolam renang", "swimming pool",
		"dokumentasi", "documentation",
		"ps 3", "ps 4", "ps 5",
	},
}

var categoryRules = map[string][]string{
	"food":     {"food", "foods", "makanan", "main course", "maincourse", "appetizer", "snack", "snacks", "cemilan", "camilan", "dessert", "desserts", "desert", "pastry", "bakery", "side dish", "hotplate", "sayuran", "bites", "breakfast"},
	"beverage": {"beverage", "beverages", "baverage", "drink", "drinks", "minuman", "coffee", "coffe", "kopi", "tea", "teh", "juice", "jus", "mocktail", "milkshake", "squash", "flavor blend", "refresher", "refreshing", "refreshment", "float", "coconut series", "blend"},
	"activity": {"activity", "activities", "aktivitas", "aktifitas", "aktifity", "wahana", "permainan", "tiket", "ticket"},
}

// Normalize treats punctuation as a word boundary, so "mi" cannot match a
// fragment of "mineral" and "tea" cannot match "steak".
func normalize(s string) string {
	// Quantities may be written without spaces ("2KOPI-2DONAT", "ESPRESSO10L").
	// Treat letter/digit transitions as boundaries; activity rules also cover PS4.
	var text strings.Builder
	previousDigit, previousLetter := false, false
	for _, r := range strings.ToLower(s) {
		if (previousDigit && unicode.IsLetter(r)) || (previousLetter && unicode.IsNumber(r)) {
			text.WriteByte(' ')
		}
		text.WriteRune(r)
		previousDigit = unicode.IsNumber(r)
		previousLetter = unicode.IsLetter(r)
	}
	return strings.Join(strings.FieldsFunc(text.String(), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	}), " ")
}

func hasPhrase(text, phrase string) bool {
	return strings.Contains(" "+text+" ", " "+phrase+" ")
}

func evidence(text string, rules map[string][]string) map[string]bool {
	result := map[string]bool{}
	for kind, phrases := range rules {
		for _, phrase := range phrases {
			if hasPhrase(text, phrase) {
				result[kind] = true
				break
			}
		}
	}
	return result
}

func knownKind(kind string) bool {
	return kind == "food" || kind == "beverage" || kind == "activity"
}

// Fingerprint binds a suggestion to its inputs and the rules version. Recipe
// ordering is canonical so a different SQL/JSON array order cannot invalidate
// an otherwise identical preview.
func Fingerprint(in models.ProductClassificationInput) string {
	in.RecipeEvidence = append([]models.ProductClassificationRecipe{}, in.RecipeEvidence...)
	sort.Slice(in.RecipeEvidence, func(i, j int) bool {
		a, b := in.RecipeEvidence[i], in.RecipeEvidence[j]
		if a.Position != b.Position {
			return a.Position < b.Position
		}
		return a.RecipeID < b.RecipeID
	})
	data, _ := json.Marshal(struct {
		Version string `json:"rules_version"`
		models.ProductClassificationInput
	}{Version: rulesVersion, ProductClassificationInput: in})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func Classify(in models.ProductClassificationInput) models.ProductClassificationRow {
	r := models.ProductClassificationRow{
		ProductID: in.ProductID, OutletID: in.OutletID, OutletName: in.OutletName,
		Name: in.Name, CategoryName: in.CategoryName, CurrentKind: in.CurrentKind,
		Status: "needs_review", Confidence: "none", Fingerprint: Fingerprint(in),
		Reason: "Belum ada bukti nama atau kategori yang cukup jelas; tinjau manual.",
	}
	if knownKind(in.CurrentKind) {
		r.Status, r.Confidence = "existing", "high"
		r.Reason = "Jenis yang sudah ditetapkan pada profil atau resep dipertahankan."
		return r
	}
	if in.CurrentKind != "" {
		r.Reason = "Jenis lama tidak dikenal; tinjau manual tanpa menimpa nilai yang ada."
		return r
	}
	// Owner-defined business grouping includes every catalog item at this
	// outlet, including supporting items such as raincoats. Match the full
	// outlet name, never product/category text or another outlet's prefix.
	if strings.EqualFold(strings.Join(strings.Fields(in.OutletName), " "), "ATV Overland") {
		r.SuggestedKind, r.Status, r.Confidence = "activity", "suggested", "high"
		r.Reason = "Ketentuan outlet ATV Overland: seluruh produk dikelompokkan sebagai Aktivitas."
		return r
	}
	for _, recipe := range in.RecipeEvidence {
		if recipe.MenuKind != "" {
			r.Reason = "Ada jenis pada resep tetapi jenis efektif produk kosong; periksa profil dan urutan resep."
			return r
		}
	}
	name, category := normalize(in.Name), normalize(in.CategoryName)
	// Staff menus may abbreviate a bundled drink after the meal (e.g. a
	// chicken/noodle item with "amer" or "jasmine"). Neither word establishes
	// the bundle's kind, so preserve it for human review instead of assuming food.
	if hasPhrase(name, "amer") || hasPhrase(name, "jasmine") {
		for _, meal := range []string{"ayam", "chicken", "mie", "mi", "noodle", "noodles", "nasi", "rice", "bihun", "bakmi"} {
			if hasPhrase(name, meal) {
				r.Reason = "Nama makanan memuat singkatan atau nama minuman; periksa apakah ini paket gabungan."
				return r
			}
		}
	}
	for _, ambiguous := range []string{"bundling", "jas hujan", "raincoat", "merchandise", "kaos", "t shirt", "custom payment"} {
		if hasPhrase(name, ambiguous) {
			r.Reason = "Paket gabungan, barang, atau pembayaran umum memerlukan peninjauan manual."
			return r
		}
	}
	// These Indonesian dishes can be served as a drink or a bowl dessert.
	for _, ambiguous := range []string{"es buah", "es campur", "es teler", "sop buah", "soup buah"} {
		if hasPhrase(name, ambiguous) {
			r.Reason = "Menu dapat berupa minuman atau hidangan penutup; tinjau cara penyajiannya."
			return r
		}
	}
	// A recognized main dish cannot determine an entire bundle when a later
	// component is unknown. Do not infer those components from the category.
	if strings.Contains(in.Name, "+") {
		comboKind := ""
		for _, component := range strings.Split(in.Name, "+") {
			componentKinds := evidence(normalize(component), nameRules)
			if len(componentKinds) != 1 {
				r.Reason = "Ada komponen paket yang belum jelas atau memuat beberapa jenis; tinjau seluruh paket."
				return r
			}
			for kind := range componentKinds {
				if comboKind != "" && comboKind != kind {
					r.Reason = "Paket menggabungkan beberapa jenis; tinjau manual."
					return r
				}
				comboKind = kind
			}
		}
	}
	signals := evidence(name, nameRules)
	if (hasPhrase(category, "add on") || hasPhrase(category, "komplemen")) && len(signals) == 0 {
		r.Reason = "Kategori tambahan dapat berisi topping atau bahan; nama tambahan perlu ditinjau."
		return r
	}
	// Matcha can be a pastry flavour. Only ignore that single drink cue when
	// an explicit pastry name and a food category agree, with no combo marker.
	foodCategory := evidence(category, categoryRules)["food"] || evidence(category, nameRules)["food"]
	pastry := false
	for _, phrase := range []string{"eclair", "donut", "donuts", "donat", "cake"} {
		pastry = pastry || hasPhrase(name, phrase)
	}
	if pastry && foodCategory && !strings.ContainsAny(in.Name, "+&") && !hasPhrase(name, "paket") && !hasPhrase(name, "package") {
		withoutMatcha := strings.ReplaceAll(" "+name+" ", " matcha ", " ")
		if !evidence(strings.TrimSpace(withoutMatcha), nameRules)["beverage"] {
			delete(signals, "beverage")
		}
	}
	for kind := range evidence(category, categoryRules) {
		signals[kind] = true
	}
	for kind := range evidence(category, nameRules) {
		signals[kind] = true
	}
	if len(signals) > 1 {
		r.Reason = "Nama atau kategori memuat beberapa jenis (termasuk paket campuran); tinjau manual."
		return r
	}
	if len(signals) == 0 {
		if len(evidence(normalize(in.Description), nameRules)) > 0 {
			r.Reason = "Petunjuk hanya terdapat pada deskripsi; konfirmasikan jenis secara manual."
		}
		return r
	}
	for kind := range signals {
		r.SuggestedKind = kind
	}
	r.Status, r.Confidence = "suggested", "high"
	r.Reason = "Nama atau kategori memiliki petunjuk jenis yang jelas tanpa bukti bertentangan."
	return r
}

func Report(inputs []models.ProductClassificationInput) models.ProductClassificationReport {
	r := models.ProductClassificationReport{Rows: make([]models.ProductClassificationRow, 0, len(inputs))}
	for _, input := range inputs {
		row := Classify(input)
		r.Rows = append(r.Rows, row)
		r.Totals.Total++
		kind := row.CurrentKind
		switch row.Status {
		case "existing":
			r.Totals.Existing++
		case "suggested":
			r.Totals.Suggested++
			kind = row.SuggestedKind
		default:
			r.Totals.NeedsReview++
		}
		switch kind {
		case "food":
			r.Totals.Food++
		case "beverage":
			r.Totals.Beverage++
		case "activity":
			r.Totals.Activity++
		}
	}
	return r
}
