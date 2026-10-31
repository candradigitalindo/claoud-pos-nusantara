package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

func SavePrinter(outletID string, req models.PushPrinterRequest) (string, error) {
	req.LocalID = strings.TrimSpace(req.LocalID)
	if req.LocalID == "" {
		req.LocalID = NewULID()
	}
	tx, err := database.DB.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	cloudID, err := reservePOSID(tx, "printer", outletID, req.LocalID)
	if err != nil {
		return "", err
	}
	err = tx.QueryRow(
		`INSERT INTO cloud_printers (id, local_id, outlet_id, name, ip_address, port,
			printer_type, paper_size, is_active, is_deleted, synced_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
		ON CONFLICT (outlet_id, local_id) DO UPDATE SET
			name = EXCLUDED.name,
			ip_address = EXCLUDED.ip_address,
			port = EXCLUDED.port,
			printer_type = EXCLUDED.printer_type,
			paper_size = EXCLUDED.paper_size,
			is_active = EXCLUDED.is_active,
			is_deleted = EXCLUDED.is_deleted,
			updated_at = NOW(),
			synced_at = NOW()
		RETURNING id`,
		cloudID, req.LocalID, outletID, req.Name, req.IPAddress, req.Port,
		req.PrinterType, req.PaperSize, req.IsActive, req.IsDeleted,
	).Scan(&cloudID)
	if err != nil {
		return "", err
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}
	go logSync(outletID, "push_printer", "printer", 1, "success", "")
	return cloudID, nil
}

func GetOutletPrinters(outletID string) ([]models.CloudPrinter, error) {
	rows, err := database.DB.Query(
		`SELECT id, local_id, outlet_id, name, ip_address, port,
			printer_type, paper_size, is_active, is_deleted, created_at, updated_at, synced_at
		FROM cloud_printers
		WHERE outlet_id = $1 AND is_deleted = false
		ORDER BY printer_type, name`,
		outletID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	printers := make([]models.CloudPrinter, 0)
	for rows.Next() {
		var p models.CloudPrinter
		if err := rows.Scan(&p.ID, &p.LocalID, &p.OutletID, &p.Name, &p.IPAddress,
			&p.Port, &p.PrinterType, &p.PaperSize, &p.IsActive, &p.IsDeleted,
			&p.CreatedAt, &p.UpdatedAt, &p.SyncedAt); err != nil {
			return nil, err
		}
		printers = append(printers, p)
	}
	return printers, rows.Err()
}

func SaveCashierShift(outletID string, req models.PushCashierShiftRequest) (string, error) {
	req.LocalID = strings.TrimSpace(req.LocalID)
	if req.LocalID == "" {
		req.LocalID = NewULID()
	}
	tx, err := database.DB.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	cloudID, err := reservePOSID(tx, "cashier_shift", outletID, req.LocalID)
	if err != nil {
		return "", err
	}
	if req.PreviousShiftID != "" {
		req.PreviousShiftID, err = reservePOSID(tx, "cashier_shift", outletID, req.PreviousShiftID)
		if err != nil {
			return "", err
		}
	}

	var closedAt interface{}
	if req.ClosedAt != "" {
		closed, valid := parseTimeStrict(req.ClosedAt)
		if !valid {
			return "", Invalid("waktu tutup shift tidak valid")
		}
		closedAt = closed
		// A closing timestamp is terminal even on legacy payloads whose status
		// was accidentally left as open. Never invent a replacement timestamp.
		req.Status = "closed"
	}

	var reportJSON interface{}
	if req.Report != nil {
		if b, err := json.Marshal(req.Report); err == nil {
			reportJSON = string(b)
		}
	}

	// opened_at kolomnya NOT NULL. Payload dengan opened_at kosong/tak terparse
	// sebelumnya gagal INSERT dan di-retry device selamanya (item macet permanen
	// di outbox). Fallback ke waktu sekarang + catat, daripada menolak shift-nya.
	openedAt := parseTime(req.OpenedAt)
	if _, ok := openedAt.(time.Time); !ok {
		var original time.Time
		if lookupErr := tx.QueryRow(`SELECT opened_at FROM cloud_cashier_shifts WHERE outlet_id=$1 AND local_id=$2`, outletID, req.LocalID).Scan(&original); lookupErr == nil {
			openedAt = original
		} else if lookupErr == sql.ErrNoRows {
			log.Printf("SaveCashierShift: opened_at tidak valid (outlet=%s local_id=%s), fallback ke NOW", outletID, req.LocalID)
			openedAt = time.Now().UTC()
		} else {
			return "", lookupErr
		}
	}

	err = tx.QueryRow(
		`INSERT INTO cloud_cashier_shifts (id, local_id, outlet_id, opened_by, opened_at,
			opening_cash, closed_at, closed_by, closing_cash, closing_card, closing_qris,
			closing_transfer, carry_over_cash, previous_shift_id, handover_to, status, notes, report,
			created_at, updated_at, synced_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, NOW(), NOW(), NOW())
		ON CONFLICT (outlet_id, local_id) DO UPDATE SET
			opened_by = EXCLUDED.opened_by,
			opened_at = EXCLUDED.opened_at,
			opening_cash = EXCLUDED.opening_cash,
			closed_at = COALESCE(EXCLUDED.closed_at, cloud_cashier_shifts.closed_at),
			closed_by = EXCLUDED.closed_by,
			closing_cash = EXCLUDED.closing_cash,
			closing_card = EXCLUDED.closing_card,
			closing_qris = EXCLUDED.closing_qris,
			closing_transfer = EXCLUDED.closing_transfer,
			carry_over_cash = EXCLUDED.carry_over_cash,
			previous_shift_id = EXCLUDED.previous_shift_id,
			handover_to = EXCLUDED.handover_to,
			status = EXCLUDED.status,
			notes = EXCLUDED.notes,
			report = EXCLUDED.report,
			updated_at = NOW(),
			synced_at = NOW()
		WHERE cloud_cashier_shifts.status <> 'closed' AND cloud_cashier_shifts.closed_at IS NULL
		RETURNING id`,
		cloudID, req.LocalID, outletID, req.OpenedBy, openedAt,
		req.OpeningCash, closedAt, req.ClosedBy, req.ClosingCash, req.ClosingCard,
		req.ClosingQris, req.ClosingTransfer, req.CarryOverCash, req.PreviousShiftID,
		req.HandoverTo, req.Status, req.Notes, reportJSON,
	).Scan(&cloudID)

	if err == sql.ErrNoRows {
		// The upsert locked an already closed shift. Without a revision on the
		// wire, a retried close cannot be distinguished from an older snapshot.
		// Acknowledge its stable identity, preserving the accepted close/report.
		// Repair a legacy open+closed_at status only on this explicit sync;
		// never rewrite its event times, money, or report, and do not notify twice.
		err = tx.QueryRow(`UPDATE cloud_cashier_shifts SET status='closed',synced_at=NOW()
			WHERE outlet_id=$1 AND local_id=$2 AND (status='closed' OR closed_at IS NOT NULL)
			RETURNING id`, outletID, req.LocalID).Scan(&cloudID)
		if err != nil {
			return "", err
		}
		if err := tx.Commit(); err != nil {
			return "", err
		}
		return cloudID, nil
	}
	if err != nil {
		return "", err
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}

	go logSync(outletID, "push_cashier_shift", "cashier_shift", 1, "success", "")
	BroadcastSync("cashier_shift", outletID)
	if req.Status == "closed" {
		go NotifyCashierShiftClosed(outletID, cloudID, req)
	}
	return cloudID, nil
}

// SaveOrderItemVoid menyimpan audit void item (hapus item dari order belum bayar).
// Idempotent per local_id + voided_at; void parsial berikutnya punya waktu berbeda.
func SaveOrderItemVoid(outletID string, req models.PushOrderItemVoidRequest) (string, error) {
	cloudID := NewULID()
	at, valid := parseTimeStrict(req.VoidedAt)
	if !valid {
		return "", fmt.Errorf("waktu void item tidak valid")
	}
	req.VoidedAt = at.UTC().Format(time.RFC3339Nano)

	tx, err := database.DB.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if err = lockPOSStock(tx, outletID); err != nil {
		return "", err
	}
	if req.OrderID != "" {
		req.OrderID, err = reservePOSID(tx, "order", outletID, req.OrderID)
		if err != nil {
			return "", err
		}
	}
	if err = applyItemVoidStock(tx, outletID, req); err != nil {
		return "", fmt.Errorf("waste pembatalan item: %w", err)
	}
	err = tx.QueryRow(
		`INSERT INTO order_item_voids (id, local_id, outlet_id, order_id, table_number,
			product_name, category_id, qty, price, subtotal, waiter_name,
			voided_by, void_reason, voided_at, created_at, synced_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NOW(), NOW())
		ON CONFLICT (outlet_id, local_id, voided_at) DO UPDATE SET synced_at = NOW()
		RETURNING id`,
		cloudID, req.LocalID, outletID, req.OrderID, req.TableNumber,
		req.ProductName, req.CategoryID, req.Qty, req.Price, req.Subtotal,
		req.WaiterName, req.VoidedBy, req.VoidReason, parseTime(req.VoidedAt),
	).Scan(&cloudID)

	if err != nil {
		return "", err
	}

	if err = tx.Commit(); err != nil {
		return "", err
	}
	go logSync(outletID, "push_order_item_void", "order_item_void", 1, "success", "")
	BroadcastSync("order_item_void", outletID)
	return cloudID, nil
}

// SaveOrderItemTitipan menyimpan audit event titip (park) / tarik (pull) satu item
// Meja Titipan sebagai log append-only. Idempotent: retry outbox dengan
// (item, aksi, waktu) yang sama tidak menggandakan baris.
func SaveOrderItemTitipan(outletID string, req models.PushOrderItemTitipanRequest) (string, error) {
	var id int64
	err := database.DB.QueryRow(
		`INSERT INTO order_item_titipan_logs
			(outlet_id, local_id, action, product_name, qty, price, subtotal,
			 category_id, source_table, target_table, performed_by, performed_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (outlet_id, local_id, action, performed_at) DO NOTHING
		RETURNING id`,
		outletID, req.LocalID, req.Action, req.ProductName, req.Qty, req.Price, req.Subtotal,
		req.CategoryID, req.SourceTable, req.TargetTable, req.By, parseTime(req.At),
	).Scan(&id)
	// ON CONFLICT DO NOTHING → tidak ada baris dikembalikan pada duplikat (idempotent).
	if err != nil && err != sql.ErrNoRows {
		return "", err
	}

	go logSync(outletID, "push_order_item_titipan", "order_item_titipan", 1, "success", "")
	BroadcastSync("order_item_titipan", outletID)
	return req.LocalID, nil
}

func SaveCashMovement(outletID string, req models.PushCashMovementRequest) (string, error) {
	req.LocalID = strings.TrimSpace(req.LocalID)
	if req.LocalID == "" {
		req.LocalID = NewULID()
	}
	tx, err := database.DB.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	// Allocate the parent first, matching SaveCashierShift's lock order.
	if req.ShiftID != "" {
		req.ShiftID, err = reservePOSID(tx, "cashier_shift", outletID, req.ShiftID)
		if err != nil {
			return "", err
		}
	}
	cloudID, err := reservePOSID(tx, "cash_movement", outletID, req.LocalID)
	if err != nil {
		return "", err
	}
	err = tx.QueryRow(
		`INSERT INTO cloud_cash_movements (id, local_id, outlet_id, shift_id,
			movement_type, amount, counterpart_name, note, created_at, synced_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		ON CONFLICT (outlet_id, local_id) DO UPDATE SET
			shift_id = EXCLUDED.shift_id,
			movement_type = EXCLUDED.movement_type,
			amount = EXCLUDED.amount,
			counterpart_name = EXCLUDED.counterpart_name,
			note = EXCLUDED.note,
			synced_at = NOW()
		RETURNING id`,
		cloudID, req.LocalID, outletID, req.ShiftID,
		req.MovementType, req.Amount, req.CounterpartName, req.Note,
		parseTime(req.CreatedAt),
	).Scan(&cloudID)

	if err != nil {
		return "", err
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}

	go logSync(outletID, "push_cash_movement", "cash_movement", 1, "success", "")
	BroadcastSync("cash_movement", outletID)
	return cloudID, nil
}

func SaveAnalytics(outletID string, req models.PushAnalyticsRequest) (string, error) {
	summaryJSON, _ := json.Marshal(req.Summary)
	cloudID := NewULID()
	err := database.DB.QueryRow(
		`INSERT INTO cloud_analytics (id, outlet_id, outlet_code, date, summary)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (outlet_id, date) DO UPDATE SET
			summary = EXCLUDED.summary,
			updated_at = NOW()
		RETURNING id`,
		cloudID, outletID, req.OutletCode, req.Date, string(summaryJSON),
	).Scan(&cloudID)

	if err != nil {
		return "", err
	}

	go logSync(outletID, "push_analytics", "analytics", 1, "success", "")
	return cloudID, nil
}

func GetAnalytics(outletID, startDate, endDate string) ([]models.CloudAnalytics, error) {
	query := `SELECT id, outlet_id, outlet_code, date::text, summary::text,
		created_at, updated_at FROM cloud_analytics WHERE outlet_id = $1`
	args := []interface{}{outletID}

	if startDate != "" && endDate != "" {
		query += " AND date BETWEEN $2 AND $3"
		args = append(args, startDate, endDate)
	}
	query += " ORDER BY date DESC"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	analytics := make([]models.CloudAnalytics, 0)
	for rows.Next() {
		var a models.CloudAnalytics
		if err := rows.Scan(&a.ID, &a.OutletID, &a.OutletCode, &a.Date,
			&a.Summary, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		analytics = append(analytics, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return analytics, nil
}

func ProcessBatchSync(outletID string, req models.BatchSyncRequest) models.BatchSyncResponse {
	resp := models.BatchSyncResponse{
		Processed: len(req.Items),
		SyncedAt:  time.Now().UTC().Format(time.RFC3339),
	}

	for _, item := range req.Items {
		result := models.BatchSyncResult{
			EntityType: item.EntityType,
			Status:     "success",
		}

		if dataMap, ok := item.Data.(map[string]interface{}); ok {
			if eventID, ok := dataMap["event_id"].(string); ok {
				result.EventID = eventID
			}
			normalizeSyncFields(dataMap, item.EntityType)
			item.Data = dataMap
		}

		dataBytes, _ := json.Marshal(item.Data)

		switch item.EntityType {
		case "order":
			var orderReq models.PushOrderRequest
			if err := json.Unmarshal(dataBytes, &orderReq); err != nil {
				result.Status = "failed"
				result.Error = "Invalid order data: " + err.Error()
				resp.Failed++
			} else {
				if orderReq.OutletCode == "" {
					orderReq.OutletCode = req.OutletCode
				}
				result.LocalID = orderReq.LocalID
				if cloudID, err := SaveOrder(outletID, orderReq); err != nil {
					result.Status = "failed"
					result.Error = err.Error()
					resp.Failed++
				} else {
					result.CloudID = cloudID
					resp.Success++
				}
			}

		case "transaction":
			var txReq models.PushTransactionRequest
			if err := json.Unmarshal(dataBytes, &txReq); err != nil {
				result.Status = "failed"
				result.Error = "Invalid transaction data: " + err.Error()
				resp.Failed++
			} else {
				if txReq.OutletCode == "" {
					txReq.OutletCode = req.OutletCode
				}
				result.LocalID = txReq.LocalID
				if cloudID, err := SaveTransaction(outletID, txReq); err != nil {
					result.Status = "failed"
					result.Error = err.Error()
					resp.Failed++
				} else {
					result.CloudID = cloudID
					resp.Success++
				}
			}

		case "product":
			var prodReq models.PushProductRequest
			if err := json.Unmarshal(dataBytes, &prodReq); err != nil {
				result.Status = "failed"
				result.Error = "Invalid product data: " + err.Error()
				resp.Failed++
			} else {
				result.LocalID = prodReq.LocalID
				if item.Operation == "delete" {
					if err := DeleteProduct(outletID, prodReq.LocalID); err != nil {
						result.Status = "failed"
						result.Error = err.Error()
						resp.Failed++
					} else {
						resp.Success++
					}
				} else {
					if cloudID, err := SaveProduct(outletID, prodReq); err != nil {
						result.Status = "failed"
						result.Error = err.Error()
						resp.Failed++
					} else {
						result.CloudID = cloudID
						resp.Success++
					}
				}
			}

		case "additional_charge":
			var chargeReq models.PushAdditionalChargeRequest
			if err := json.Unmarshal(dataBytes, &chargeReq); err != nil {
				result.Status = "failed"
				result.Error = "Invalid additional charge data: " + err.Error()
				resp.Failed++
			} else {
				result.LocalID = chargeReq.LocalID
				if cloudID, err := SaveAdditionalCharge(outletID, chargeReq); err != nil {
					result.Status = "failed"
					result.Error = err.Error()
					resp.Failed++
				} else {
					result.CloudID = cloudID
					resp.Success++
				}
			}

		case "product_addon":
			var addonReq models.PushProductAddonRequest
			if err := json.Unmarshal(dataBytes, &addonReq); err != nil {
				result.Status = "failed"
				result.Error = "Invalid product addon data: " + err.Error()
				resp.Failed++
			} else {
				result.LocalID = addonReq.LocalID
				if item.Operation == "delete" {
					if err := DeleteProductAddon(outletID, addonReq.LocalID); err != nil {
						result.Status = "failed"
						result.Error = err.Error()
						resp.Failed++
					} else {
						result.CloudID = addonReq.LocalID
						resp.Success++
					}
				} else if cloudID, err := SaveProductAddon(outletID, addonReq); err != nil {
					result.Status = "failed"
					result.Error = err.Error()
					resp.Failed++
				} else {
					result.CloudID = cloudID
					resp.Success++
				}
			}

		case "category":
			dataMap := make(map[string]interface{})
			if err := json.Unmarshal(dataBytes, &dataMap); err != nil {
				result.Status = "failed"
				result.Error = "Invalid category data: " + err.Error()
				resp.Failed++
			} else {
				localID := ""
				if v, ok := dataMap["local_id"].(string); ok && v != "" {
					localID = v
				} else if v, ok := dataMap["id"].(string); ok && v != "" {
					localID = v
				}
				name := ""
				if v, ok := dataMap["name"].(string); ok {
					name = v
				}
				codePrefix := ""
				if v, ok := dataMap["code_prefix"].(string); ok {
					codePrefix = v
				}
				version := 1
				if v, ok := dataMap["version"].(float64); ok {
					version = int(v)
				}

				catReq := models.PushCategoryRequest{
					LocalID:    localID,
					Name:       name,
					CodePrefix: codePrefix,
					Version:    version,
				}

				result.LocalID = localID
				if item.Operation == "delete" {
					if err := DeleteCategory(outletID, name); err != nil {
						result.Status = "failed"
						result.Error = err.Error()
						resp.Failed++
					} else {
						resp.Success++
					}
				} else {
					if cloudID, err := SaveCategory(outletID, catReq); err != nil {
						result.Status = "failed"
						result.Error = err.Error()
						resp.Failed++
					} else {
						result.CloudID = cloudID
						resp.Success++
					}
				}
			}

		case "printer":
			var printerReq models.PushPrinterRequest
			if err := json.Unmarshal(dataBytes, &printerReq); err != nil {
				result.Status = "failed"
				result.Error = "Invalid printer data: " + err.Error()
				resp.Failed++
			} else {
				result.LocalID = printerReq.LocalID
				if item.Operation == "delete" {
					printerReq.IsDeleted = true
				}
				if cloudID, err := SavePrinter(outletID, printerReq); err != nil {
					result.Status = "failed"
					result.Error = err.Error()
					resp.Failed++
				} else {
					result.CloudID = cloudID
					resp.Success++
				}
			}

		case "cashier_shift":
			var shiftReq models.PushCashierShiftRequest
			if err := json.Unmarshal(dataBytes, &shiftReq); err != nil {
				result.Status = "failed"
				result.Error = "Invalid cashier_shift data: " + err.Error()
				resp.Failed++
			} else {
				result.LocalID = shiftReq.LocalID
				if cloudID, err := SaveCashierShift(outletID, shiftReq); err != nil {
					result.Status = "failed"
					result.Error = err.Error()
					resp.Failed++
				} else {
					result.CloudID = cloudID
					resp.Success++
				}
			}

		case "order_item_void":
			var voidReq models.PushOrderItemVoidRequest
			if err := json.Unmarshal(dataBytes, &voidReq); err != nil {
				result.Status = "failed"
				result.Error = "Invalid order_item_void data: " + err.Error()
				resp.Failed++
			} else {
				result.LocalID = voidReq.LocalID
				if cloudID, err := SaveOrderItemVoid(outletID, voidReq); err != nil {
					result.Status = "failed"
					result.Error = err.Error()
					resp.Failed++
				} else {
					result.CloudID = cloudID
					resp.Success++
				}
			}

		case "order_item_titipan":
			var tReq models.PushOrderItemTitipanRequest
			if err := json.Unmarshal(dataBytes, &tReq); err != nil {
				result.Status = "failed"
				result.Error = "Invalid order_item_titipan data: " + err.Error()
				resp.Failed++
			} else {
				result.LocalID = tReq.LocalID
				if cloudID, err := SaveOrderItemTitipan(outletID, tReq); err != nil {
					result.Status = "failed"
					result.Error = err.Error()
					resp.Failed++
				} else {
					result.CloudID = cloudID
					resp.Success++
				}
			}

		case "cashier_cash_movement":
			var movReq models.PushCashMovementRequest
			if err := json.Unmarshal(dataBytes, &movReq); err != nil {
				result.Status = "failed"
				result.Error = "Invalid cashier_cash_movement data: " + err.Error()
				resp.Failed++
			} else {
				result.LocalID = movReq.LocalID
				if cloudID, err := SaveCashMovement(outletID, movReq); err != nil {
					result.Status = "failed"
					result.Error = err.Error()
					resp.Failed++
				} else {
					result.CloudID = cloudID
					resp.Success++
				}
			}

		default:
			localID := ""
			if dataMap, ok := item.Data.(map[string]interface{}); ok {
				if v, ok := dataMap["local_id"].(string); ok {
					localID = v
				} else if v, ok := dataMap["id"].(string); ok {
					localID = v
				}
			}
			result.LocalID = localID
			result.Status = "failed"
			result.Error = fmt.Sprintf("Unsupported entity_type: %s", item.EntityType)
			resp.Failed++
		}

		// Persistenkan kegagalan per-item: tanpa ini server hanya menyimpan agregat
		// "N of M items failed" dan item yang macet tak pernah bisa didiagnosis.
		if result.Status == "failed" {
			go logSync(outletID, "batch_sync_item", item.EntityType, 1, "failed",
				result.LocalID+": "+result.Error)
		}

		resp.Results = append(resp.Results, result)
	}

	syncStatus := "success"
	syncErr := ""
	if resp.Failed > 0 {
		if resp.Success == 0 {
			syncStatus = "failed"
		} else {
			syncStatus = "partial"
		}
		syncErr = fmt.Sprintf("%d of %d items failed", resp.Failed, resp.Processed)
	}
	go logSync(outletID, "batch_sync", "batch", resp.Processed, syncStatus, syncErr)
	return resp
}

func GetUpdatesSince(outletID, since string) (*models.UpdatesResponse, error) {
	sinceRaw := parseTime(since)
	var sinceTime interface{}
	if t, ok := sinceRaw.(time.Time); ok {
		// Kolom updated_at bertipe timestamp tanpa zona berisi wall-clock UTC;
		// t.Local() menggeser cursor sebesar offset zona OS host (kebetulan benar
		// hanya bila host UTC). Selalu bandingkan dalam UTC.
		sinceTime = t.UTC()
	} else {
		sinceTime = sinceRaw
	}
	resp := &models.UpdatesResponse{
		Products:       make([]models.UpdateEntity, 0),
		Categories:     make([]models.UpdateEntity, 0),
		Deleted:        make([]models.DeletedEntity, 0),
		SyncCheckpoint: time.Now().UTC().Format(time.RFC3339),
	}

	rows, err := database.DB.Query(
		`SELECT id, local_id, name, COALESCE(code,''), COALESCE(description,''),
			COALESCE((SELECT c.local_id FROM cloud_categories c WHERE c.outlet_id=cloud_products.outlet_id AND c.id=cloud_products.category_id),
			 (SELECT i.local_id FROM pos_entity_ids i WHERE i.entity_type='category' AND i.outlet_id=cloud_products.outlet_id AND i.cloud_id=cloud_products.category_id), category_id,''), COALESCE(category_name,''),
			price, COALESCE(photo_url,''), COALESCE(mk.menu_kind,''), version, updated_at::text
		FROM cloud_products LEFT JOIN product_menu_kinds mk ON mk.product_id=cloud_products.id
		WHERE outlet_id = $1 AND updated_at > $2 AND is_deleted = false
		ORDER BY updated_at ASC`,
		outletID, sinceTime,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var e models.UpdateEntity
		var price float64
		if err := rows.Scan(&e.CloudID, &e.LocalID, &e.Name, &e.Code, &e.Description,
			&e.CategoryID, &e.CategoryName,
			&price, &e.PhotoURL, &e.MenuKind, &e.Version, &e.UpdatedAt); err != nil {
			return nil, err
		}
		e.Price = &price
		e.Action = "update"
		resp.Products = append(resp.Products, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	catRows, err := database.DB.Query(
		`SELECT id, COALESCE(local_id,''), name, version, updated_at::text
		FROM cloud_categories
		WHERE outlet_id = $1 AND updated_at > $2 AND is_deleted = false
		ORDER BY updated_at ASC`,
		outletID, sinceTime,
	)
	if err != nil {
		return nil, err
	}
	defer catRows.Close()
	for catRows.Next() {
		var e models.UpdateEntity
		if err := catRows.Scan(&e.CloudID, &e.LocalID, &e.Name, &e.Version, &e.UpdatedAt); err != nil {
			return nil, err
		}
		e.Action = "update"
		resp.Categories = append(resp.Categories, e)
	}
	if err := catRows.Err(); err != nil {
		return nil, err
	}

	delProdRows, err := database.DB.Query(
		`SELECT id, local_id, updated_at::text
		FROM cloud_products WHERE outlet_id = $1 AND is_deleted = true AND updated_at > $2`,
		outletID, sinceTime,
	)
	if err != nil {
		return nil, err
	}
	defer delProdRows.Close()
	for delProdRows.Next() {
		var d models.DeletedEntity
		d.EntityType = "product"
		if err := delProdRows.Scan(&d.CloudID, &d.LocalID, &d.DeletedAt); err != nil {
			return nil, err
		}
		resp.Deleted = append(resp.Deleted, d)
	}
	if err := delProdRows.Err(); err != nil {
		return nil, err
	}

	delCatRows, err := database.DB.Query(
		`SELECT id, COALESCE(local_id,''), updated_at::text
		FROM cloud_categories WHERE outlet_id = $1 AND is_deleted = true AND updated_at > $2`,
		outletID, sinceTime,
	)
	if err != nil {
		return nil, err
	}
	defer delCatRows.Close()
	for delCatRows.Next() {
		var d models.DeletedEntity
		d.EntityType = "category"
		if err := delCatRows.Scan(&d.CloudID, &d.LocalID, &d.DeletedAt); err != nil {
			return nil, err
		}
		resp.Deleted = append(resp.Deleted, d)
	}
	if err := delCatRows.Err(); err != nil {
		return nil, err
	}

	return resp, nil
}

func GetRestoreData(outletID string) (*models.RestoreResponse, error) {
	resp := &models.RestoreResponse{
		OpenShifts:        make([]models.CloudCashierShift, 0),
		UnpaidOrders:      make([]models.CloudOrder, 0),
		CashMovements:     make([]models.CloudCashMovement, 0),
		AdditionalCharges: make([]models.RestoreAdditionalCharge, 0),
		RestoredAt:        time.Now().UTC().Format(time.RFC3339),
	}

	// 1. Open cashier shifts
	shiftRows, err := database.DB.Query(
		`SELECT id, local_id, outlet_id, opened_by, opened_at, opening_cash,
			closed_at, COALESCE(closed_by,''), COALESCE(closing_cash,0), COALESCE(closing_card,0),
			COALESCE(closing_qris,0), COALESCE(closing_transfer,0), COALESCE(carry_over_cash,0),
			COALESCE((SELECT p.local_id FROM cloud_cashier_shifts p
				WHERE p.outlet_id=s.outlet_id AND p.id=s.previous_shift_id),
				(SELECT i.local_id FROM pos_entity_ids i WHERE i.entity_type='cashier_shift'
				AND i.outlet_id=s.outlet_id AND i.cloud_id=s.previous_shift_id),previous_shift_id,''), COALESCE(handover_to,''),
			status, COALESCE(notes,''), created_at, updated_at, synced_at
		FROM cloud_cashier_shifts s
		WHERE outlet_id = $1 AND status = 'open' AND closed_at IS NULL
		ORDER BY opened_at ASC`,
		outletID,
	)
	if err != nil {
		return nil, fmt.Errorf("query open shifts: %w", err)
	}
	defer shiftRows.Close()

	for shiftRows.Next() {
		var s models.CloudCashierShift
		if err := shiftRows.Scan(
			&s.ID, &s.LocalID, &s.OutletID, &s.OpenedBy, &s.OpenedAt, &s.OpeningCash,
			&s.ClosedAt, &s.ClosedBy, &s.ClosingCash, &s.ClosingCard,
			&s.ClosingQris, &s.ClosingTransfer, &s.CarryOverCash,
			&s.PreviousShiftID, &s.HandoverTo,
			&s.Status, &s.Notes, &s.CreatedAt, &s.UpdatedAt, &s.SyncedAt,
		); err != nil {
			return nil, fmt.Errorf("scan shift: %w", err)
		}
		resp.OpenShifts = append(resp.OpenShifts, s)
	}
	if err := shiftRows.Err(); err != nil {
		return nil, err
	}

	// 2. A completed transaction is authoritative even if its final paid order
	// snapshot arrived late or never arrived. Resolve legacy local references
	// within this outlet, preferring canonical IDs when both forms could match.
	orderRows, err := database.DB.Query(
		`SELECT id, local_id, outlet_id, outlet_code,
			COALESCE(table_number,''), COALESCE(customer_name,''),
			COALESCE(customer_phone,''),COALESCE(created_by,''),COALESCE(orderer_name,''),COALESCE(is_holding,false),
			pax, total_amount, status,
			COALESCE(items::text,'[]'), COALESCE(payment_info::text,'{}'),
			version, created_at, updated_at, synced_at
		FROM cloud_orders o
		WHERE outlet_id = $1
			AND lower(btrim(COALESCE(payment_info->>'payment_status','unpaid'))) <> 'paid'
			AND lower(btrim(COALESCE(status,''))) NOT IN ('paid','completed','cancelled','canceled','voided')
			AND NULLIF(btrim(payment_info->>'voided_at'),'') IS NULL
			AND NOT EXISTS (
				SELECT 1 FROM cloud_transactions t
				WHERE t.outlet_id=o.outlet_id AND (t.order_id=o.id OR t.order_id=o.local_id)
				AND (SELECT parent.id FROM cloud_orders parent
					WHERE parent.outlet_id=t.outlet_id AND (parent.id=t.order_id OR parent.local_id=t.order_id)
					ORDER BY (parent.id=t.order_id) DESC LIMIT 1)=o.id
			)
		ORDER BY created_at ASC`,
		outletID,
	)
	if err != nil {
		return nil, fmt.Errorf("query unpaid orders: %w", err)
	}
	defer orderRows.Close()

	for orderRows.Next() {
		var o models.CloudOrder
		if err := orderRows.Scan(
			&o.ID, &o.LocalID, &o.OutletID, &o.OutletCode,
			&o.TableNumber, &o.CustomerName,
			&o.CustomerPhone, &o.CreatedBy, &o.OrdererName, &o.IsHolding,
			&o.Pax, &o.TotalAmount, &o.Status,
			&o.Items, &o.PaymentInfo,
			&o.Version, &o.CreatedAt, &o.UpdatedAt, &o.SyncedAt,
		); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}
		resp.UnpaidOrders = append(resp.UnpaidOrders, o)
	}
	if err := orderRows.Err(); err != nil {
		return nil, err
	}

	// 3. Cash movements from open shifts
	if len(resp.OpenShifts) > 0 {
		movRows, err := database.DB.Query(
			`SELECT m.id, m.local_id, m.outlet_id, parent.local_id,
					m.movement_type, m.amount,
					COALESCE(m.counterpart_name,''), COALESCE(m.note,''), m.created_at
				FROM cloud_cash_movements m
				JOIN LATERAL (SELECT s.local_id,s.status,s.closed_at FROM cloud_cashier_shifts s
					WHERE s.outlet_id=m.outlet_id AND (s.id=m.shift_id OR s.local_id=m.shift_id)
					ORDER BY (s.id=m.shift_id) DESC LIMIT 1) parent ON true
				WHERE m.outlet_id = $1 AND parent.status='open' AND parent.closed_at IS NULL
				ORDER BY m.created_at ASC,m.id`, outletID,
		)
		if err != nil {
			return nil, fmt.Errorf("query cash movements: %w", err)
		}
		defer movRows.Close()

		for movRows.Next() {
			var m models.CloudCashMovement
			if err := movRows.Scan(
				&m.ID, &m.LocalID, &m.OutletID, &m.ShiftID,
				&m.MovementType, &m.Amount,
				&m.CounterpartName, &m.Note, &m.CreatedAt,
			); err != nil {
				return nil, fmt.Errorf("scan cash movement: %w", err)
			}
			resp.CashMovements = append(resp.CashMovements, m)
		}
		if err := movRows.Err(); err != nil {
			return nil, err
		}
	}

	chargeRows, err := database.DB.Query(`SELECT local_id,name,charge_type,value,is_active
		FROM cloud_additional_charges WHERE outlet_id=$1 ORDER BY local_id`, outletID)
	if err != nil {
		return nil, fmt.Errorf("query restore charges: %w", err)
	}
	defer chargeRows.Close()
	for chargeRows.Next() {
		var c models.RestoreAdditionalCharge
		if err := chargeRows.Scan(&c.LocalID, &c.Name, &c.ChargeType, &c.Value, &c.IsActive); err != nil {
			return nil, fmt.Errorf("scan restore charge: %w", err)
		}
		resp.AdditionalCharges = append(resp.AdditionalCharges, c)
	}
	if err := chargeRows.Err(); err != nil {
		return nil, err
	}

	resp.Summary = models.RestoreSummary{
		OpenShiftCount:    len(resp.OpenShifts),
		UnpaidOrderCount:  len(resp.UnpaidOrders),
		CashMovementCount: len(resp.CashMovements),
	}

	go logSync(outletID, "restore", "restore", 1, "success", "")
	return resp, nil
}

func GetConflicts(outletID string) ([]models.SyncConflict, error) {
	rows, err := database.DB.Query(
		`SELECT id, outlet_id, entity_type, COALESCE(entity_local_id,''),
			COALESCE(entity_cloud_id::text,''), COALESCE(conflict_field,''),
			COALESCE(cloud_value,''), COALESCE(local_value,''),
			COALESCE(cloud_version,0), COALESCE(local_version,0),
			COALESCE(resolution,'pending'), COALESCE(resolved_by,''),
			resolved_at::text, COALESCE(notes,''), created_at::text
		FROM sync_conflicts WHERE outlet_id = $1
		ORDER BY created_at DESC`,
		outletID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	conflicts := make([]models.SyncConflict, 0)
	for rows.Next() {
		var c models.SyncConflict
		if err := rows.Scan(&c.ID, &c.OutletID, &c.EntityType,
			&c.EntityLocalID, &c.EntityCloudID, &c.ConflictField,
			&c.CloudValue, &c.LocalValue, &c.CloudVersion, &c.LocalVersion,
			&c.Resolution, &c.ResolvedBy, &c.ResolvedAt, &c.Notes, &c.CreatedAt); err != nil {
			return nil, err
		}
		conflicts = append(conflicts, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return conflicts, nil
}

func ResolveConflict(outletID, conflictID string, req models.ResolveConflictRequest) error {
	result, err := database.DB.Exec(
		`UPDATE sync_conflicts
		SET resolution = $1, resolved_by = $2, notes = $3, resolved_at = NOW()
		WHERE id = $4 AND outlet_id = $5`,
		req.Strategy, req.ResolvedBy, req.Notes, conflictID, outletID,
	)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("conflict not found")
	}
	return nil
}

func logSync(outletID, action, entityType string, count int, status, errMsg string) {
	_, err := database.DB.Exec(
		`INSERT INTO sync_logs (id, outlet_id, action, entity_type, entity_count, status, error_message)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		NewULID(), outletID, action, entityType, count, status, errMsg,
	)
	if err != nil {
		log.Printf("Failed to log sync: %v", err)
	}
}

func GetSyncLogs(outletID string, limit int) ([]models.SyncLog, error) {
	rows, err := database.DB.Query(
		`SELECT id, outlet_id, action, COALESCE(entity_type,''),
			entity_count, status, COALESCE(error_message,''), created_at::text
		FROM sync_logs WHERE outlet_id = $1
		ORDER BY created_at DESC LIMIT $2`,
		outletID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs := make([]models.SyncLog, 0)
	for rows.Next() {
		var l models.SyncLog
		if err := rows.Scan(&l.ID, &l.OutletID, &l.Action, &l.EntityType,
			&l.EntityCount, &l.Status, &l.ErrorMessage, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return logs, nil
}
