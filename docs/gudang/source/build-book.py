import json, pathlib, html, hashlib
source=pathlib.Path(__file__).resolve().parent
out=source.parent if source.name=='source' else pathlib.Path('/home/candra/projects/cloud-pos/docs/gudang')
read=lambda n:json.loads((source/n).read_text())
front=read('front-pages.json');rules=read('rule-pages.json');guides=read('guide-pages.json');end=read('end-pages.json')
for p in rules:p.setdefault('kind','rule')
for p in guides:p.setdefault('kind','guide')
toc1={'id':'daftar-isi','title':'Daftar isi · aturan & SOP','kind':'toc','subtitle':'Jenis gudang, pengendalian stok, dan ketentuan setiap transaksi.'}
toc2={'id':'daftar-isi-langkah','title':'Daftar isi · cara menggunakan','kind':'toc','subtitle':'Langkah operasional dan screenshot tampilan aplikasi.'}
pages=front+[toc1,toc2]+rules+guides+end
lookup={p['id']:i+1 for i,p in enumerate(pages)}
assert len(lookup)==len(pages),'ID halaman duplikat'
def rows(ps):
 return ''.join(f'<a class="toc-row" href="#{p["id"]}"><span>{html.escape(p["title"])}</span><span class="num">{lookup[p["id"]]}</span></a>' for p in ps)
toc1['html']='<h3>Aturan dan pola aplikasi</h3>'+rows(rules)+'<h3>Checklist dan rujukan</h3>'+rows(end)
half=(len(guides)+1)//2
toc2['html']='<div class="cols"><div>'+rows(guides[:half])+'</div><div>'+rows(guides[half:])+'</div></div>'
chunks=[];images=[]
for i,p in enumerate(pages):
 kind=p.get('kind','appendix');content=p['html'];head=''
 if kind!='cover':
  section={'rule':'Bagian I · Aturan Gudang','guide':'Bagian II · Cara Menggunakan','toc':'Navigasi Dokumen','intro':'Pendahuluan'}.get(kind,'Lampiran Operasional')
  head=f'<div class="eyebrow">{section}</div><h1>{html.escape(p["title"])}</h1>'
  if p.get('subtitle'):head+=f'<p class="subtitle">{html.escape(p["subtitle"])}</p>'
  if p.get('route'):head+=f'<p class="route">{html.escape(p["route"])}</p>'
 if p.get('image'):
  path=out/'screenshots'/p['image'];assert path.is_file(),path
  images.append({'file':p['image'],'sha256':hashlib.sha256(path.read_bytes()).hexdigest()})
  head+=f'<figure><img src="screenshots/{html.escape(p["image"])}" alt="{html.escape(p["title"])}"><figcaption>Screenshot aplikasi · data simulasi terisolasi · 27 Oktober 2026</figcaption></figure>'
 footer=f'<footer class="footer"><span>Cloud POS · Operasional Gudang</span><a href="#daftar-isi">Daftar isi</a><span>{i+1} / {len(pages)}</span></footer>'
 chunks.append(f'<section id="{p["id"]}" class="page {kind}"><div class="content">{head}<div class="body">{content}</div></div>{footer}</section>')
css=(source/'book.css').read_text()
css+='\n.toc .toc-row{font-size:8.7pt;padding:1.3mm 0;line-height:1.28}.toc h3{margin-top:3mm}\n'
(out/'panduan-operasional-gudang.html').write_text('<!DOCTYPE html><html lang="id"><head><meta charset="UTF-8"><title>Panduan Operasional Gudang — Cloud POS</title><style>'+css+'</style></head><body>'+''.join(chunks)+'</body></html>')
manifest={'title':'Panduan Operasional Gudang — Cloud POS','date':'2026-10-27','revision':'Revisi 4 — Fokus PPIC: kontrol bahan, opname, kedaluwarsa','screen_data':'simulasi terisolasi; screenshot revisi sebelumnya, perubahan label dijelaskan dalam naskah','pages':[{'page':i+1,**{k:v for k,v in p.items() if k!='html'}} for i,p in enumerate(pages)],'images':images}
(out/'manifest.json').write_text(json.dumps(manifest,ensure_ascii=False,indent=2))
print(f'{len(pages)} halaman, {len(images)} screenshot')
