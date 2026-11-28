export const DELIVERY_STATUS_LABELS = {
  draft: 'Draft',
  in_transit: 'Dalam Pengantaran',
  partial_received: 'Diterima Sebagian',
  received: 'Diterima',
  cancelled: 'Dibatalkan',
}

export const DISPATCH_MODE_LABELS = {
  logistics: 'Petugas Logistik',
  vendor_direct: 'Vendor Langsung',
}

export const LINE_KIND_LABELS = {
  dapur: 'Bahan / Dapur',
  perlengkapan: 'Perlengkapan / Aset',
  projek: 'Material Projek',
}

export function deliveryStatusLabel(s) {
  return DELIVERY_STATUS_LABELS[s] || s || '-'
}

export function deliveryStatusClass(s) {
  const map = {
    draft: 'bg-gray-100 text-gray-700',
    in_transit: 'bg-amber-100 text-amber-800',
    partial_received: 'bg-blue-100 text-blue-700',
    received: 'bg-emerald-100 text-emerald-700',
    cancelled: 'bg-red-100 text-red-600',
  }
  return map[s] || 'bg-gray-100 text-gray-600'
}

export function dispatchModeLabel(m) {
  return DISPATCH_MODE_LABELS[m] || m || '-'
}

export function purchasingGroupLabel(g) {
  if (g === 'bahan_baku' || g === 'bahan') return 'Purchasing Bahan Baku'
  if (g === 'barang_jasa' || g === 'barang' || g === 'jasa') return 'Purchasing Barang & Jasa'
  return ''
}

export function purchasingGroupShort(g) {
  if (g === 'bahan_baku' || g === 'bahan') return 'Kel. 1 · Bahan Baku'
  if (g === 'barang_jasa' || g === 'barang' || g === 'jasa') return 'Kel. 2 · Barang & Jasa'
  return ''
}

export function purchasingGroupBadgeClass(g) {
  if (g === 'bahan_baku' || g === 'bahan') return 'bg-amber-100 text-amber-900 border-amber-300'
  return 'bg-blue-100 text-blue-900 border-blue-300'
}

/**
 * Cetak Surat Jalan Pengantaran Logistik (A4 / Save as PDF) melalui jendela
 * cetak terpisah agar tidak terpengaruh CSS aplikasi.
 */
export function printDeliveryNote(d) {
  if (!d) return { blocked: false }
  const esc = (v) => String(v ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]))
  const fmtDT = (v) => {
    if (!v) return '-'
    const dt = new Date(v)
    if (Number.isNaN(dt.getTime())) return String(v)
    return dt.toLocaleString('id-ID', { timeZone: 'Asia/Jakarta', day: '2-digit', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' })
  }
  const items = d.items || []
  const totalPlanned = items.reduce((s, it) => s + (Number(it.qty_planned) || 0), 0)
  const totalReceived = items.reduce((s, it) => s + (Number(it.qty_received) || 0), 0)

  const rows = items.map((it, i) => `
    <tr>
      <td class="num">${i + 1}</td>
      <td>
        <div class="mono"><strong>${esc(it.pr_number || '-')}</strong></div>
        <div class="sub-cell">${esc(it.vendor_name || '-')}</div>
      </td>
      <td>
        <strong>${esc(it.item_name)}</strong>
        <div class="sub-cell">${esc(LINE_KIND_LABELS[it.line_kind] || it.line_kind || '-')}${it.outlet_name ? ' · ' + esc(it.outlet_name) : ''}${it.notes ? ' · ' + esc(it.notes) : ''}</div>
      </td>
      <td class="num"><strong>${it.qty_planned}</strong> ${esc(it.unit || '')}</td>
      <td class="num">${it.qty_received > 0 ? `${it.qty_received} ${esc(it.unit || '')}` : '________'}</td>
      <td class="num">[ &nbsp; ]</td>
    </tr>`).join('')

  const html = `<!doctype html><html><head><meta charset="utf-8"><title>Surat Jalan ${esc(d.delivery_number)}</title>
  <style>
    @page { size: A4; margin: 14mm; }
    body { font-family: system-ui, -apple-system, "Segoe UI", sans-serif; font-size: 10.5pt; color: #111; margin: 0; }
    .hdr { border-bottom: 1.5pt solid #111; padding-bottom: 4mm; margin-bottom: 5mm; display: flex; justify-content: space-between; align-items: flex-end; gap: 10mm; }
    .hdr h1 { font-size: 15pt; margin: 0 0 1mm; text-transform: uppercase; letter-spacing: .5pt; }
    .hdr .num-box { text-align: right; }
    .hdr .sj-num { font-family: ui-monospace, monospace; font-size: 12pt; font-weight: 700; }
    .badge { display: inline-block; padding: 1mm 2.5mm; border: .3mm solid #444; border-radius: 2mm; font-size: 8.5pt; font-weight: 600; margin-top: 1mm; }
    .meta-grid { display: flex; gap: 8mm; margin-bottom: 5mm; font-size: 9.5pt; }
    .meta-grid table { flex: 1; border-collapse: collapse; }
    .meta-grid td { padding: 1mm 0; vertical-align: top; }
    .meta-grid td:first-child { width: 32mm; color: #555; }
    table.items { width: 100%; border-collapse: collapse; font-size: 9.5pt; }
    table.items th, table.items td { border: .3mm solid #777; padding: 2mm; text-align: left; vertical-align: top; }
    table.items th { background: #f3f4f6; font-weight: 700; }
    table.items tfoot td { background: #f9fafb; font-weight: 700; }
    .num { text-align: center; white-space: nowrap; }
    .mono { font-family: ui-monospace, monospace; font-size: 8.5pt; }
    .sub-cell { font-size: 8pt; color: #555; margin-top: .5mm; }
    .note { margin-top: 5mm; padding: 2.5mm 3mm; border: .3mm solid #ccc; border-radius: 1.5mm; font-size: 9.5pt; background: #fafafa; }
    .sign { display: flex; justify-content: space-between; gap: 6mm; margin-top: 14mm; font-size: 9.5pt; page-break-inside: avoid; }
    .sign > div { flex: 1; text-align: center; }
    .sign .role { font-weight: 600; color: #333; }
    .sign .line { margin-top: 20mm; border-top: .3mm solid #333; padding-top: 1.5mm; font-size: 9pt; }
    .foot { margin-top: 8mm; font-size: 8pt; color: #666; text-align: right; }
    @media screen { body { background: #f3f4f6; padding: 10mm; } .sheet { background: #fff; max-width: 210mm; margin: 0 auto; padding: 12mm; box-shadow: 0 2px 12px rgba(0,0,0,.08); } }
  </style></head><body><div class="sheet">
  <div class="hdr">
    <div>
      <h1>Surat Jalan Pengantaran Barang</h1>
      <div style="font-size:9.5pt;color:#444;">${esc(purchasingGroupLabel(d.purchasing_group) || 'Pengantaran Logistik & Sinkronisasi Pengadaan (Purchasing)')}</div>
    </div>
    <div class="num-box">
      <div class="sj-num">${esc(d.delivery_number)}</div>
      <div class="badge">${esc(dispatchModeLabel(d.dispatch_mode))} · ${esc(deliveryStatusLabel(d.status))}</div>
    </div>
  </div>

  <div class="meta-grid">
    <table>
      <tr><td>Kelompok</td><td>: <strong>${esc(purchasingGroupLabel(d.purchasing_group) || '-')}</strong></td></tr>
      <tr><td>Tanggal Kirim</td><td>: <strong>${esc(d.delivery_date || '-')}</strong></td></tr>
      <tr><td>Asal / Pengirim</td><td>: ${esc(d.origin_label || 'Purchasing / Vendor')}</td></tr>
      <tr><td>No. Pengadaan</td><td>: <span class="mono">${esc((d.pr_numbers || []).join(', ') || '-')}</span></td></tr>
      <tr><td>Dibuat Oleh</td><td>: ${esc(d.created_by || '-')}</td></tr>
    </table>
    <table>
      <tr><td>Tujuan</td><td>: <strong>${esc(d.destination_label || '-')}</strong></td></tr>
      <tr><td>Petugas / Kurir</td><td>: <strong>${esc(d.courier_name || (d.dispatch_mode === 'vendor_direct' ? 'Pengantar Vendor' : '-'))}</strong></td></tr>
      ${d.vehicle_info ? `<tr><td>Kendaraan</td><td>: ${esc(d.vehicle_info)}</td></tr>` : ''}
      ${d.dispatched_at ? `<tr><td>Berangkat</td><td>: ${esc(fmtDT(d.dispatched_at))}</td></tr>` : ''}
      ${d.received_at ? `<tr><td>Diterima</td><td>: ${esc(d.received_by || '-')} · ${esc(fmtDT(d.received_at))}</td></tr>` : ''}
    </table>
  </div>

  <table class="items">
    <thead>
      <tr>
        <th class="num" style="width:9mm;">No</th>
        <th style="width:40mm;">No. Pengadaan / Vendor</th>
        <th>Nama Barang &amp; Keterangan</th>
        <th class="num" style="width:24mm;">Qty Dibawa</th>
        <th class="num" style="width:24mm;">Qty Diterima</th>
        <th class="num" style="width:16mm;">Ceklis</th>
      </tr>
    </thead>
    <tbody>${rows || '<tr><td colspan="6" class="num">— Tidak ada item —</td></tr>'}</tbody>
    <tfoot>
      <tr>
        <td colspan="3">Total: ${items.length} jenis barang</td>
        <td class="num">${totalPlanned}</td>
        <td class="num">${totalReceived > 0 ? totalReceived : '-'}</td>
        <td></td>
      </tr>
    </tfoot>
  </table>

  ${d.notes ? `<div class="note"><strong>Catatan Pengantaran:</strong> ${esc(d.notes)}</div>` : ''}

  <div class="sign">
    <div>
      <div class="role">Diserahkan Oleh</div>
      <div class="sub-cell">(Purchasing / Vendor)</div>
      <div class="line">${esc(d.created_by || '________________________')}</div>
    </div>
    <div>
      <div class="role">Pembawa Barang</div>
      <div class="sub-cell">(${esc(dispatchModeLabel(d.dispatch_mode))})</div>
      <div class="line">${esc(d.courier_name || '________________________')}</div>
    </div>
    <div>
      <div class="role">Diterima Oleh</div>
      <div class="sub-cell">(${esc(d.destination_label || 'Gudang / Aset / Outlet')})</div>
      <div class="line">${esc(d.received_by || '________________________')}</div>
    </div>
  </div>
  <div class="foot">Dicetak pada ${esc(fmtDT(new Date().toISOString()))} WIB</div>
  </div>
  <script>window.onload = function () { window.focus(); window.print(); }<\/script>
  </body></html>`

  const win = window.open('', '_blank', 'width=940,height=740')
  if (!win) return { blocked: true }
  win.document.open()
  win.document.write(html)
  win.document.close()
  return { blocked: false }
}
