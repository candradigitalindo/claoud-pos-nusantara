package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"math"
	"sort"
	"time"

	"github.com/lib/pq"
)

type reservationClearingChange struct {
	at     time.Time
	amount float64 // deposits minus DP usage; refunds/voids reverse their source
}

type reservationClearingSource struct {
	id, outletID, customer, transactionID string
	settled                               time.Time
	changes                               []reservationClearingChange
}

// Clearing changes have fixed event dates. A later POS payment reverses the
// earlier clearing entry on its own date instead of rewriting the closing day.
func loadFinanceReservationClearing(from, to, outlet string, scope []string) ([]models.FinanceOperation, error) {
	location := GetTimezoneLocation()
	start, err := time.ParseInLocation("2006-01-02", from, location)
	if err != nil {
		return nil, err
	}
	end, err := time.ParseInLocation("2006-01-02", to, location)
	if err != nil || end.Before(start) {
		return nil, Invalid("rentang tanggal tidak valid")
	}
	end = end.AddDate(0, 0, 1)
	if outlet != "" {
		if !financeInScope(outlet, scope) {
			return nil, ErrFinanceAccess
		}
		scope = []string{outlet}
	}
	filter := ""
	args := []interface{}{}
	if scope != nil {
		filter = ` AND rv.outlet_id=ANY($1::text[])`
		args = append(args, pq.Array(scope))
	}
	rows, err := database.DB.Query(`SELECT rv.id,TRIM(rv.outlet_id),rv.customer_name,
		COALESCE(rv.pos_transaction_id,''),rv.settled_at FROM reservations rv
		WHERE rv.status='done' AND rv.settled_at IS NOT NULL`+filter, args...)
	if err != nil {
		return nil, err
	}
	sources := map[string]*reservationClearingSource{}
	byTransaction := map[string]string{}
	for rows.Next() {
		var source reservationClearingSource
		if err := rows.Scan(&source.id, &source.outletID, &source.customer, &source.transactionID, &source.settled); err != nil {
			rows.Close()
			return nil, err
		}
		source.changes = []reservationClearingChange{{at: source.settled}}
		sources[source.id] = &source
		if source.transactionID != "" {
			byTransaction[source.outletID+":"+source.transactionID] = source.id
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return []models.FinanceOperation{}, nil
	}
	rows, err = database.DB.Query(`SELECT p.reservation_id,p.paid_at,
		CASE WHEN p.type='refund' THEN -p.amount ELSE p.amount END
		FROM reservation_payments p JOIN reservations rv ON rv.id=p.reservation_id AND rv.outlet_id=p.outlet_id
		WHERE rv.status='done' AND p.status='validated'`+filter, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id string
		var change reservationClearingChange
		if err := rows.Scan(&id, &change.at, &change.amount); err != nil {
			rows.Close()
			return nil, err
		}
		if source := sources[id]; source != nil {
			source.changes = append(source.changes, change)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	entities, err := loadFinancePOSEntities(outlet, scope)
	if err != nil {
		return nil, err
	}
	orders, aliases := map[string]*financePOSEntity{}, map[string]*financePOSEntity{}
	for _, entity := range entities {
		if entity.Kind == "order" {
			state := entity.Revisions[0].State
			orders[state.OutletID+":"+state.ID], aliases[state.OutletID+":"+state.LocalID] = entity, entity
		}
	}
	addUsage := func(id, outletID string, entity *financePOSEntity, evidence financePOSEvidence, voided time.Time) {
		source := sources[id]
		if source == nil || source.outletID != outletID {
			return
		}
		// Use the exact same dated evidence/correction stream as POS postings.
		// In particular, cash -> DP corrections affect clearing when observed,
		// without moving an already established historical cash payment.
		financePOSEmitPayments(entity, voided, evidence,
			func(financePOSState, time.Time) string { return glAccReceivable },
			func(at time.Time, _, _ string, _ financePOSState, debit, credit string, amount float64, _, _ string) {
				usage := 0.0
				if debit == glAccCustomerDep {
					usage += amount
				}
				if credit == glAccCustomerDep {
					usage -= amount
				}
				if usage != 0 && !at.IsZero() {
					source.changes = append(source.changes, reservationClearingChange{at: at, amount: -usage})
				}
			})
	}
	linked := map[*financePOSEntity]bool{}
	for _, entity := range entities {
		if entity.Kind != "transaction" {
			continue
		}
		state := entity.Revisions[len(entity.Revisions)-1].State
		order := orders[state.OutletID+":"+state.OrderID]
		if order == nil {
			order = aliases[state.OutletID+":"+state.OrderID]
		}
		id := byTransaction[state.OutletID+":"+state.ID]
		if id == "" {
			id = byTransaction[state.OutletID+":"+state.LocalID]
		}
		voided := financePOSVoidAt(entity)
		if order != nil {
			linked[order] = true
			candidate := order.Revisions[len(order.Revisions)-1].State.PaymentInfo.Restore.ReservationID
			if source := sources[candidate]; source != nil && source.outletID == state.OutletID {
				id = candidate
			}
			if voided.IsZero() {
				voided = financePOSVoidAt(order)
			}
		}
		addUsage(id, state.OutletID, entity, func(s financePOSState) ([]financePOSPayment, bool) {
			return financePOSPayments(s), len(s.Payments) > 0
		}, voided)
	}
	for _, entity := range orders {
		if !linked[entity] {
			state := entity.Revisions[len(entity.Revisions)-1].State
			addUsage(state.PaymentInfo.Restore.ReservationID, state.OutletID, entity,
				financePOSOrderPayments, financePOSVoidAt(entity))
		}
	}
	ops := []models.FinanceOperation{}
	for _, source := range sources {
		sort.SliceStable(source.changes, func(i, j int) bool { return source.changes[i].at.Before(source.changes[j].at) })
		var net, pending float64
		for index := 0; index < len(source.changes); {
			at := source.changes[index].at
			for index < len(source.changes) && source.changes[index].at.Equal(at) {
				net = round2(net + source.changes[index].amount)
				index++
			}
			if at.Before(source.settled) {
				continue
			}
			target := round2(math.Max(0, net))
			delta := round2(target - pending)
			pending = target
			if delta == 0 || at.Before(start) || !at.Before(end) {
				continue
			}
			op := models.FinanceOperation{ID: "reservation_pending:" + source.id + ":" + at.UTC().Format(time.RFC3339Nano),
				Source: "reservation_pending", SourceID: source.id, Reference: source.transactionID,
				At: at, Date: at.In(location).Format("2006-01-02"), OutletID: source.outletID,
				Description:  "Perubahan DP menunggu sinkronisasi POS: " + source.customer,
				Counterparty: source.customer, Status: "posted", Postings: []models.FinancePosting{}}
			financePair(&op, glAccCustomerDep, "2-350", delta)
			if delta > 0 {
				op.ReviewNote = "DP belum terpakai pada bukti POS pada tanggal kejadian ini; periksa riwayat sinkronisasi reservasi."
			}
			ops = append(ops, op)
		}
	}
	sort.SliceStable(ops, func(i, j int) bool {
		if ops[i].At.Equal(ops[j].At) {
			return ops[i].ID < ops[j].ID
		}
		return ops[i].At.Before(ops[j].At)
	})
	return ops, nil
}
