"""Generate original synthetic fixtures for the Go Image Toolbox PRD.
No external network or copyright-dependent source files are required.
Generated visual files are illustrative tests, NOT ICC or full GIF-disposal standard corpora.
"""
from PIL import Image, ImageDraw, ImageFont
from pathlib import Path
import hashlib, json, math

root=Path(__file__).resolve().parent/'fixtures'
root.mkdir(parents=True,exist_ok=True)
font_path='/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc'
font=ImageFont.truetype(font_path,26) if Path(font_path).exists() else ImageFont.load_default()
font_big=ImageFont.truetype(font_path,48) if Path(font_path).exists() else ImageFont.load_default()

# Odd-size crop coverage fixture
im=Image.new('RGB',(1001,667),'white'); d=ImageDraw.Draw(im)
for j in range(7):
 for i in range(10):
  x0=i*100; y0=j*95
  d.rectangle((x0,y0,min(x0+99,1000),min(y0+94,666)), fill=((i*27)%256,(j*31)%256,((i+j)*17)%256))
d.rectangle((0,0,1000,666),outline='white',width=2)
d.text((25,15),'1001 × 667',font=font,fill='white')
im.save(root/'odd_1001x667_crop.png')

# Transparency and antialiased edge fixture (RGBA)
im=Image.new('RGBA',(512,512),(0,0,0,0)); d=ImageDraw.Draw(im)
for r in range(218,40,-1):
 alpha=int(255*(218-r)/180)
 d.ellipse((256-r,256-r,256+r,256+r),fill=(45,142,232,alpha))
d.rounded_rectangle((126,158,386,354),radius=52,fill=(255,115,70,175))
im.save(root/'alpha_512x512.png')

# RGB gradient with narrow lines
w,h=1280,720; im=Image.new('RGB',(w,h)); pix=im.load()
for y in range(h):
 for x in range(w):
  pix[x,y]=(int(255*x/(w-1)),int(255*y/(h-1)),int(255*(x+y)/(w+h-2)))
d=ImageDraw.Draw(im)
for x in range(0,w,37): d.line((x,0,x,h-1),fill=(255,255,255),width=1)
im.save(root/'gradient_1280x720.png')

# 1-bit e-paper reference
w,h=400,300; im=Image.new('RGB',(w,h),'white'); d=ImageDraw.Draw(im)
for y in range(0,h,10):
 for x in range(0,w,10):
  if (x//10+y//10)%3==0: d.rectangle((x,y,x+9,y+9),fill='black')
d.rectangle((30,55,370,245),fill='white',outline='black',width=5)
d.text((60,100),'400 × 300',font=font_big,fill='black')
im.save(root/'bw_400x300_reference.png')

# Screenshots and text edges
im=Image.new('RGB',(1600,900),(245,247,250)); d=ImageDraw.Draw(im)
d.rounded_rectangle((30,30,1570,870),radius=16,fill='white',outline=(210,215,225),width=2)
d.rectangle((30,30,1570,105),fill=(38,80,126))
d.text((80,49),'图片工具箱 · 中文细节与边缘测试',font=font_big,fill='white')
labels=['格式转换  图片裁剪  压缩预览','细线条、汉字、数字 0123456789','AVIF · WebP · PNG · JPEG','目标文件大小 200 KB；原图 2.4 MB','RGBA / ICC / EXIF / GIF']
for i,label in enumerate(labels):
 y=150+i*120
 d.text((90,y),label,font=font,fill=(34,42,54))
 d.line((90,y+45,1510,y+45),fill=(200,210,222),width=1)
d.rectangle((1300,125,1500,300),fill=(20,60,80));d.rounded_rectangle((1320,145,1480,280),radius=12,fill=(12,188,190))
im.save(root/'screenshot_text_1600x900.png')

# PNG icon alpha
im=Image.new('RGBA',(256,256),(0,0,0,0));d=ImageDraw.Draw(im)
d.rounded_rectangle((28,28,228,228),radius=52,fill=(0,128,250,255))
d.ellipse((75,75,181,181),fill=(255,255,255,255))
d.ellipse((102,102,154,154),fill=(0,128,250,255))
im.save(root/'icon_alpha_256.png')

# Simple GIF: varying durations, always complete frames. Not a disposal-3 stress fixture.
frames=[]
for i in range(6):
 f=Image.new('RGB',(320,240),'white');dd=ImageDraw.Draw(f)
 dd.rectangle((20,20,300,220),outline=(20,20,20),width=3)
 x=35+i*40
 dd.ellipse((x,80,x+70,150),fill=(30+i*20,90,180-i*20))
 dd.text((22,30),f'frame {i+1}',font=font,fill=(20,20,20))
 frames.append(f)
frames[0].save(root/'animated_basic_6frames.gif',save_all=True,append_images=frames[1:],duration=[90,150,240,100,330,120],loop=0,optimize=False)

manifest={"origin":"Original synthetic fixtures generated locally for Go Image Toolbox PRD", "warning":"Not a substitute for real ICC/EXIF/complex GIF disposal/APNG/WebP/GPU test corpora", "files": []}
for p in sorted(root.iterdir()):
 if not p.is_file(): continue
 raw=p.read_bytes()
 with Image.open(p) as img:
  dims=img.size; fmt=img.format; mode=img.mode; frames=getattr(img,'n_frames',1)
 manifest['files'].append({"name":p.name,"sha256":hashlib.sha256(raw).hexdigest(),"bytes":len(raw),"format":fmt,"dimensions":list(dims),"mode":mode,"frames":frames})
(Path(__file__).resolve().parent/'fixture_manifest.json').write_text(json.dumps(manifest,ensure_ascii=False,indent=2),encoding='utf-8')
print('Created',len(manifest['files']),'fixtures')
for f in manifest['files']: print(f['name'], f['bytes'], f['dimensions'],f['frames'])
