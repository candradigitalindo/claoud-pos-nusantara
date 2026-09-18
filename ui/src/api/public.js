import axios from 'axios'

// Bare client for public (no-auth) endpoints — no token header, no 401 redirect.
const publicClient = axios.create({ baseURL: '/api/v1', timeout: 20000 })

export const publicApi = {
  menu:    (slug)       => publicClient.get(`/public/outlets/${slug}/menu`).then(r => r.data),
  reserve: (slug, data) => publicClient.post(`/public/outlets/${slug}/reservations`, data).then(r => r.data),
  // Status reservasi + rekening tujuan DP; dipakai halaman sukses dan link cek status (?id=).
  status:  (slug, id)   => publicClient.get(`/public/outlets/${slug}/reservations/${id}`).then(r => r.data),
  // Unggah bukti transfer (multipart: file, type, amount, bank_account_id, paid_at, notes).
  pay:     (slug, id, formData) => publicClient.post(`/public/outlets/${slug}/reservations/${id}/payments`, formData,
    { headers: { 'Content-Type': 'multipart/form-data' } }).then(r => r.data),
}
