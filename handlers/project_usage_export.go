package handlers

import (
	"cloud-pos/services"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
)

func ExportProjectUsage(c *fiber.Ctx) error {
	f := excelize.NewFile()
	defer f.Close()
	sheet := "Penggunaan Barang"
	f.SetSheetName("Sheet1", sheet)
	headers := []interface{}{"Nomor laporan", "Projek", "Jenis", "Tanggal penggunaan", "Status", "Barang", "Pengadaan", "Jumlah", "Satuan", "Lokasi", "Pekerjaan/alasan", "Pelaksana", "PIC", "Pencatat", "Waktu dicatat", "Pemeriksa", "Waktu diperiksa", "Catatan pemeriksaan", "Bukti"}
	if err := f.SetSheetRow(sheet, "A1", &headers); err != nil {
		return projectUsageError(c, err)
	}
	row := 2
	for page := 1; ; page++ {
		list, total, err := services.ListProjectUsage(c.Query("project_id"), c.Query("status"), c.Query("search"), c.Query("from"), c.Query("to"), getOutletScope(c), getWorkUnitScope(c), page, 100)
		if err != nil {
			return projectUsageError(c, err)
		}
		for _, r := range list {
			reviewed := ""
			if r.ReviewedAt != nil {
				reviewed = r.ReviewedAt.In(services.GetTimezoneLocation()).Format("2006-01-02 15:04:05")
			}
			for _, it := range r.Items {
				values := []interface{}{r.ReportNumber, r.ProjectName, r.Kind, r.UsageDate, r.Status, it.Name, it.RequestNumber, it.Qty, it.Unit, r.Location, r.WorkDescription, r.Performer, r.PICName, r.CreatedBy, r.CreatedAt.In(services.GetTimezoneLocation()).Format("2006-01-02 15:04:05"), r.ReviewedBy, reviewed, r.ReviewNote, strings.Join(r.Evidence, "\n")}
				if err = f.SetSheetRow(sheet, fmt.Sprintf("A%d", row), &values); err != nil {
					return projectUsageError(c, err)
				}
				row++
			}
		}
		if page*100 >= total || len(list) == 0 {
			break
		}
	}
	f.SetColWidth(sheet, "A", "G", 22)
	f.SetColWidth(sheet, "J", "S", 28)
	f.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
	buf, err := f.WriteToBuffer()
	if err != nil {
		return projectUsageError(c, err)
	}
	c.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set("Content-Disposition", `attachment; filename="penggunaan-barang-projek.xlsx"`)
	c.Set("Cache-Control", "no-store")
	return c.Send(buf.Bytes())
}
