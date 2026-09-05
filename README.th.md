# GoShort

*[Read this in English →](README.md)*

URL shortener ที่สร้างขึ้นรอบข้อจำกัดเดียว: **`GET /:code` ต้องตอบก่อนที่อย่างอื่นจะเกิดขึ้น**

ตอน cache อุ่น redirect อ่านจาก Redis ตอบ `302` กลับไป **แล้วค่อย** ผลัก click event เข้า buffered
channel ที่ worker pool ขนาดคงที่คอยดึงไปเขียนเป็นชุด ถ้า channel เต็ม event จะถูกทิ้งและนับไว้
แทนที่จะปล่อยให้มันไปบล็อกคนที่กำลังรอถูก redirect อยู่

**Go 1.26 · Fiber · GORM · PostgreSQL · Redis · Prometheus · React · Docker**

| | |
|---|---|
| ![หน้าย่อลิงก์](docs/screenshots/01-shorten.png) | ![หน้าเฝ้าระบบ](docs/screenshots/05-dashboard.png) |
| หน้าย่อลิงก์สาธารณะ — ทุกตัวเลขอ่านสดจากโปรเซสที่กำลังรันอยู่ | Operations — เกจสดอย่างเดียว poll ทุก 2 วิ |
| ![ทะเบียนลิงก์](docs/screenshots/06-links.png) | ![สถิติรายลิงก์](docs/screenshots/07-analytics.png) |
| ทะเบียนลิงก์ — เรียง กรอง แบ่งหน้า ทำในเบราว์เซอร์ทั้งหมด | สถิติรายลิงก์ — สถานะ cache นับถอยหลังตาม TTL จริงของ Redis |
| ![ภาพรวม](docs/screenshots/10-overview.png) | ![สถิติสาธารณะ](docs/screenshots/02-public-stats.png) |
| Analytics — สรุป 14 วัน cache 60 วิ poll ทุก 30 วิ | สถิติสาธารณะ — ดูตัวเลขของลิงก์ได้โดยไม่ต้องมีบัญชี |

---

## สถาปัตยกรรม

```
                      ┌──────────────────────────────────────────────┐
   GET /:code  ─────► │ 1. Redis GET link:{code}      ── hit ──┐      │
                      │ 2. miss → Postgres → SET back to cache │      │
                      │                                        ▼      │
                      │ 3. 302 Found  ◄──────────── response goes out │
                      │                             FIRST, always     │
                      │ 4. click event ─► buffered channel (1000)     │
                      │       └─ full? drop + count, never block      │
                      └───────────────────┬──────────────────────────┘
                                          │
                          ┌───────────────▼────────────────┐
                          │  8 workers, batch 50 / 2s      │
                          │  CreateInBatches once          │
                          │  UPDATE click_count = +n       │  ← atomic, one per link per batch
                          └───────────────┬────────────────┘
                                          ▼
                                     PostgreSQL

   SIGINT ─► stop accepting ─► close(channel) ─► WaitGroup drain ─► close DB
```

React SPA ถูก build ด้วย Vite ฝังเข้า Go binary ด้วย `embed.FS` แล้วเสิร์ฟจาก origin เดียวกับ API —
container เดียวจบ ไม่ต้องมี nginx และคุกกี้ `httpOnly` ใช้งานได้โดยไม่ต้องแตะ CORS เลย

`GET /:code` เป็น wildcard ที่ root **ลำดับการลงทะเบียน route จึงสำคัญ**: static assets ก่อน แล้ว
`/api/*`, `/health`, `/metrics` ตามด้วย path ที่ SPA เป็นเจ้าของ และ `/:code` อยู่ท้ายสุดเสมอ
โค้ดที่หาไม่เจอจะได้ SPA shell พร้อมสถานะ `404` จริง ส่วน custom alias ถูกตรวจกับรายการคำสงวน
เพื่อไม่ให้ใครจองลิงก์ทับ `/admin` ได้

---

## Deployment roles

binary ตัวเดียวกันรันได้สองบทบาท ตัดสินด้วย `ADMIN_ENABLED` บน instance สาธารณะ route ของ admin
**ไม่ถูกลงทะเบียนเลย** จึงตอบ `404` ไม่ใช่ `401` — เส้นทางนั้นไม่มีอยู่จริงบน host นั้น ไม่ใช่มีอยู่
แล้วถูกป้องกันไว้ `docker-compose.yml` รันทั้งสองบทบาทจาก image เดียวกัน การแยกนี้จึงมองเห็นได้
โดยไม่ต้องรันอะไรเลย

| | `ADMIN_ENABLED=0` | `ADMIN_ENABLED=1` |
|---|---|---|
| `POST /api/admin/login` | `404` — ไม่มีใน router | `200` |
| `GET /login`, `/admin` | `404` — ตกไปที่ `/:code` | หน้าคอนโซล |
| JavaScript ของ admin | chunk ของคอนโซลไม่ถูกโหลดเลย | โหลดตอนต้องใช้ หลังวาดหน้าแรกเสร็จ |
| `GET /api/admin/me` ตอนโหลดหน้า | ไม่ยิง | ยิง |
| `/`, `/s/:code`, `/:code`, `/health`, `/metrics` | เหมือนเดิม | เหมือนเดิม |

**สิ่งที่ยืนยันเรื่อง 404 คือ test ไม่ใช่ README** — ถ้าตรงนั้นได้ `401` แปลว่า route ยังถูก mount อยู่

```
                    same image, one variable apart
   ┌──────────────────────────────────┐   ┌──────────────────────────────────┐
   │ api        ADMIN_ENABLED=0 :8080 │   │ admin      ADMIN_ENABLED=1 :8081 │
   │                                  │   │                                  │
   │  POST /api/links                 │   │  the console, /api/admin/*       │
   │  GET  /:code            302      │   │  /login, /admin                  │
   │  /health  /health/ready          │   │  /health  /health/ready          │
   │  /metrics ◄──────────────────────┼───┼── gauges read the watched        │
   │                                  │   │   instance, not this process     │
   │  click queue   per process       │   │  click queue   idle here         │
   │  metrics       per process       │   │  metrics       nothing to see    │
   └────────────────┬─────────────────┘   └────────────────┬─────────────────┘
                    │                                      │
                    └──────────────────┬───────────────────┘
                                       ▼
                         PostgreSQL          Redis
                         rows, click_count   cache, rate limits
```

ทุกอย่างที่อยู่ใต้เส้นคือของที่แชร์กัน ทุกอย่างที่อยู่ในกล่องเป็นของโปรเซสนั้นเอง คอนโซลอ่าน
`/metrics` ของ instance ที่มันเฝ้า เพราะตัวเลขพวกนั้นอยู่ในโปรเซสนั้น — คอนโซลที่รายงานตัวเองจะ
รายงานว่าไม่มีอะไรเกิดขึ้นตลอดกาล **ของจริงงานนี้เป็นหน้าที่ Prometheus** ที่ scrape ทุก instance
แล้วรวมให้ การดึงตรงแบบ hop เดียวที่นี่มีไว้ให้คอนโซลเฝ้าได้ทีละหนึ่ง instance โดยไม่ต้องยกระบบ
เก็บตัวชี้วัดขึ้นมาทั้งชุด และมันไม่ scale เกินหนึ่งตัว

โค้ดของคอนโซลถูก code-split ออกจาก bundle แรก และหน้าสาธารณะไม่ยิง `/api/admin/me` เลย
**ทั้งสองอย่างนี้ไม่ใช่ขอบเขตความปลอดภัย** — chunk ยังถูกเสิร์ฟเป็น static asset และดาวน์โหลดตรงได้
ขอบเขตจริงคือเซิร์ฟเวอร์ที่ตอบ 404

SPA รู้ว่ากำลังคุยกับบทบาทไหนจากค่าที่เซิร์ฟเวอร์แทนลงใน HTML shell ครั้งเดียวตอน boot การถาม API
จะเสีย round trip บนหน้าที่ต้องเร็วที่สุด

[ADR 0001](docs/adr/0001-deployment-roles-via-one-config-flag.md) บันทึกไว้ว่าทำไมถึงใช้ flag แทน
สอง binary และทำไมคอนโซลถึงไม่ถูกโฮสต์แยก — สรุปสั้น ๆ คือ static host ที่อยู่คนละ domain เป็น
*cross-site* คุกกี้ `SameSite=Lax` จะไม่ถูกส่งไปเลย และการทำให้ใช้ได้ต้องเปลี่ยนเป็น `SameSite=None`
บวกกับเขียน CSRF token เอง — เขียนโค้ดมากขึ้นเพื่อให้ได้ posture ที่แย่ลง

---

## รันมากกว่าหนึ่ง instance

| | แชร์ข้าม instance ไหม | |
|---|---|---|
| cache ของลิงก์ | **แชร์** — Redis | entry ที่ instance หนึ่งอุ่นไว้ เป็นของทุกตัว |
| ตัวนับ rate limit | **แชร์** — Redis `INCR` ต่อ IP | โควตาเป็นของทั้งระบบ ไม่ใช่ต่อ replica |
| `click_count` | **แชร์** — `UPDATE … SET click_count = click_count + n` | atomic ที่ฐานข้อมูล ไม่เคย read-then-write ใน Go |
| คิว click event | **ไม่แชร์** — Go channel ต่อโปรเซส | การ drop เป็นของโปรเซสนั้น และ `queue_depth` ก็เป็นความลึกของโปรเซสนั้น |
| ตัวเลข Prometheus | **ไม่แชร์** — registry อยู่ในโปรเซส | `METRICS_SOURCE_URL` ชี้คอนโซลไปที่ instance ที่มันเฝ้า การรวมมากกว่าหนึ่งตัวเป็นหน้าที่ Prometheus |
| การ migrate schema | **เข้าคิว** — Postgres advisory lock | ดูรายการสุดท้ายในหัวข้อ *บั๊กที่ test จับได้* |

health check ถูกแยกตามเส้นเดียวกัน เพราะ load balancer ตัดสินใจจากมัน:

| | เช็คอะไร | ตอน Redis ล่ม | ตอน Postgres ล่ม |
|---|---|---|---|
| `GET /health` | ไม่เช็คอะไรเลย — liveness | `200` | `200` |
| `GET /health/ready` | เช็ค Postgres และรายงาน Redis | `200 {"status":"degraded"}` | `503` |

**cache ที่ตายต้องไม่ทำให้ load balancer ว่างเปล่า** redirect fallback ไป Postgres ได้ และ
rate limiter fail-open อยู่แล้ว instance ที่ไม่มี cache จึงยังตอบถูกต้อง แค่ช้าลง การถอดทุก instance
ออกตอน Redis กระตุกคือการเปลี่ยน cache ล่มให้กลายเป็นระบบล่มทั้งหมด ส่วน Postgres ต่างออกไป —
cache miss ไม่มีอะไรให้ตกลงไปต่ำกว่านั้นแล้ว

liveness จงใจไม่แตะอะไรเลย probe ที่ ping ฐานข้อมูลจะสั่งรีสตาร์ตโปรเซสที่ยังดีอยู่ทุกครั้งที่
ฐานข้อมูลแค่ *ช้า* ซึ่งเป็นความล้มเหลวที่ liveness มีไว้ป้องกันตั้งแต่แรก

**ตัวเลขเป็นของโปรเซส คอนโซลจึงต้องถูกบอกว่าให้เฝ้าตัวไหน** บน compose redirect ลงที่ `api`
ส่วนคอนโซลรันบน `admin` — `METRICS_SOURCE_URL` ชี้ตัวหลังไปที่ตัวแรก และ**โค้ดชุดเดียวกัน**กับที่
อ่าน registry ของโปรเซสตัวเองเป็นตัวคำนวณทั้งสองทาง ไม่มีสูตรที่สองที่จะเพี้ยนออกจากกันได้
เปิดคอนโซลค้างไว้ที่ `http://localhost:8081/admin` แล้วสลับโหมด จะเห็นเกจขยับ:

```bash
GOSHORT_SYNC_MODE=1 docker compose up -d --force-recreate api admin
for i in $(seq 1 300); do curl -s -o /dev/null localhost:8080/<code>; done   # p99 ขึ้น, hit rate 0%

GOSHORT_SYNC_MODE=0 docker compose up -d --force-recreate api admin
for i in $(seq 1 300); do curl -s -o /dev/null localhost:8080/<code>; done   # p99 ลง, hit rate 100%
```

ตรงนี้**ไม่มีการอ้างตัวเลข** เพราะ `curl` แบบเรียงทีละตัวไม่ใช่การวัด ประเด็นคือความต่างนั้น
มองเห็นได้บนคอนโซลที่รันอยู่คนละโปรเซสกับตัวที่ถูกวัด ตัวเลขที่วัดจริงอยู่ในหัวข้อ *Benchmark*
ข้างล่าง ซึ่ง `load-test/run.sh` ยิง k6 ใส่โปรเซสของตัวเองที่ `:8099` และ commit raw output ไว้ให้ตรวจ

เมื่อ instance ที่ถูกเฝ้าติดต่อไม่ได้ คอนโซลตอบ `503` บอกไว้ที่แถบสถานะ และทำทุก panel จางลง
ตัวเลขจะขึ้น `—` **ไม่ใช่ `0`** เพราะศูนย์แปลว่า "ไม่มีอะไรเกิดขึ้น" ซึ่งคนละเรื่องกับ "มองไม่เห็น"
และหน้าจอเฝ้าระบบที่สับสนสองอย่างนี้แย่กว่าไม่มีหน้าจอเลย

รูปแบบข้อความของ Prometheus ถูก parse ด้วย `expfmt` ซึ่งมากับ metrics client อยู่แล้ว —
`prometheus/common` เลื่อนจาก dependency ทางอ้อมมาเป็นทางตรง และไม่มีอะไรใหม่ถูกเพิ่มเข้า build

---

## Benchmark

ทั้งสองแถวมาจาก **binary ตัวเดียวกัน** สลับด้วย `GOSHORT_SYNC_MODE` เท่านั้น ซึ่งจะข้าม Redis และ
เขียน click event แบบ synchronous อยู่ในคำขอเดียวกัน ทำซ้ำได้ด้วยคำสั่งเดียว:

```bash
docker compose up -d postgres redis
./load-test/run.sh
```

| โหมด | req/s | p50 | p95 | p99 |
|---|---:|---:|---:|---:|
| เขียนแบบ sync ไม่มี cache | 3,795 | 23.12 ms | 55.17 ms | 75.11 ms |
| เขียนแบบ async + Redis cache | **46,146** | **1.89 ms** | **3.58 ms** | **5.90 ms** |

**p99 ลดลง 92.1% throughput เพิ่มขึ้น 12.2 เท่า**

วัดบน Apple M3 Pro (11 cores, 18 GB, macOS 26.6.2) กับ Postgres 16 และ Redis 7 ใน Docker,
Go 1.26.5, k6 v2.2.0 — 100 virtual users, 30 วินาที, 5 short code, cache อุ่น raw output ของ k6
ทั้งสองรอบถูก commit ไว้ที่ [`load-test/results/`](load-test/results/) เพื่อให้ตรวจตารางย้อนกลับ
ไปหารอบที่ผลิตมันได้

ตัวเลขนี้มาจากเครื่องเดียวโดยตัวยิงโหลดอยู่บนเครื่องเดียวกับเซิร์ฟเวอร์ — มันวัด**ความต่างระหว่าง
สองโหมด** ไม่ใช่ความจุจริงของ instance ที่ deploy แล้ว

---

## วิธีรัน

```bash
cp .env.example .env
docker compose up
```

คำสั่งนี้ยก **สองบทบาทจาก image เดียวกัน** ต่างกันแค่ `ADMIN_ENABLED`:

| | | |
|---|---|---|
| `api` | http://localhost:8080 | หน้าย่อลิงก์สาธารณะและ `/:code` ทุกเส้น — `/api/admin/*` **ไม่ถูกลงทะเบียน** บนโปรเซสนี้ ตอบ `404` ไม่ใช่ `401` |
| `admin` | http://localhost:8081 | หน้าคอนโซล binary เดียวกัน image เดียวกัน ต่างกันแค่ตัวแปรเดียว |

สร้างบัญชีผู้ดูแล แล้วเข้าสู่ระบบที่ http://localhost:8081/login:

```bash
docker compose run --rm --entrypoint /seed admin
```

`--entrypoint` สำคัญ: `ENTRYPOINT` ของ image คือ `/api` ถ้าส่ง `/seed` เป็น command เฉย ๆ
มันจะสตาร์ตเซิร์ฟเวอร์แล้วเมิน argument ทิ้ง แทนที่จะรันตัว seeder

บัญชี demo: `admin@goshort.dev` / `goshort-demo` (ทั้งคู่มาจาก `.env`)

### โหมดพัฒนา

```bash
docker compose up -d postgres redis
go run ./cmd/api          # :8080 โปรเซสเดียวทำทั้งสองบทบาท (ADMIN_ENABLED ปริยายเป็นเปิด)
cd web && npm run dev     # :5173 proxy /api ไป :8080 คุกกี้จึงยังเป็น same-origin
```

---

## Test

```bash
docker compose up -d postgres redis
TEST_DATABASE_URL="postgres://goshort:goshort@localhost:5432/goshort?sslmode=disable" \
  go test -race -cover ./...
```

integration test จะข้ามตัวเองถ้าไม่ได้ตั้ง `TEST_DATABASE_URL` มี seam อยู่สี่จุด:

- **Fiber app** ขับผ่าน `app.Test` กับ Postgres จริงและ `miniredis` — ครอบทุก route, JWT guard,
  rate limit, พฤติกรรมของ cache, ลำดับ route, และ concurrency test 300 goroutine ที่พิสูจน์ว่า
  `click_count` ยังเป๊ะภายใต้ `-race`
- **worker pool** ขับผ่าน `Enqueue` / `Start` / `Shutdown` ด้วย store ที่ inject เข้าไปและตัวจับเวลา
  ที่ inject ได้ การรวม batch, การ flush ตามเวลา, การ drop ตอน back-pressure และการไล่ของค้างตอนปิด
  จึงทดสอบได้โดยไม่ต้อง sleep ใน assertion
- **`model.Migrate`** เรียกตรงกับ Postgres จริง มีไว้เพราะ migration ทำงานก่อน Fiber app ถูกสร้าง
  ใช้พิสูจน์ว่า instance ที่ boot พร้อมกันไม่ชนกันเท่านั้น
- **Playwright** เส้นหลักเส้นเดียวผ่านสแตกที่ประกอบเสร็จแล้ว

---

## บั๊กที่ test จับได้

**header ของ request ถูกทำให้เพี้ยน** ค่าที่ได้จาก `c.Get()` ชี้เข้าไปใน buffer ที่ fasthttp เอากลับไป
ใช้ซ้ำทันทีที่ handler จบ การส่งมันเข้า click channel ตรง ๆ ทำให้ worker เขียนสตริงที่ต่อกันมั่ว —
referrer ที่เป็น `https://news.ycombinator.com/` ไปโผล่ใน Postgres เป็น `https://github.com/nator.com/`
แก้ด้วย `utils.CopyString` บั๊กนี้เกิดซ้ำได้เฉพาะเมื่อยิงผ่าน listener จริงเท่านั้น เพราะ `app.Test`
สร้าง context ใหม่ทุกครั้งจึงไม่เคยแสดงอาการ

**`c.Context()` ถูกส่งให้ GORM** มันเป็น `*fasthttp.RequestCtx` ที่มีอายุแค่ช่วงชีวิตของ handler
ส่วน `database/sql` เก็บ context ไว้ใน goroutine เบื้องหลังตราบเท่าที่ rows ยังเปิดอยู่
race detector จับการเขียนหลังฟังก์ชันคืนค่าได้ ตอนนี้ทุกการเรียกฐานข้อมูลและ Redis ใช้
`c.UserContext()` แล้ว

**connection pool ไม่มีเพดาน** redirect 300 คำขอพร้อมกันเปิด connection มากกว่า `max_connections`
ของ Postgres แล้ว redirect เริ่มตอบ `500` ตอนนี้ตั้ง `SetMaxOpenConns` และเพื่อน ๆ ไว้แล้ว โหลดจึง
ไปรอคิวที่ pool แทนที่จะล้มเหลว — ซึ่งก็เป็นเหตุผลที่ชัดที่สุดว่าทำไมต้องมี cache: มันกันทราฟฟิก
ไม่ให้ไปแตะ pool เลย

**ลิงก์ที่หมดอายุยัง redirect ต่ออีกชั่วโมง** การเช็ควันหมดอายุอยู่แค่บนเส้นทาง Postgres ลิงก์ที่ถูก
cache ไว้ตอนยังไม่หมดอายุจึงตอบ `302` ต่อไปจนกว่า cache entry จะหมดอายุเอง test ที่ควรจะครอบเคสนี้
ยิงเฉพาะตอน cache เย็น ตอนนี้ entry พก `ExpiresAt` ไปด้วย และ TTL ของ Redis key ถูกจำกัดไม่ให้
เกินอายุที่เหลือของลิงก์

**สอง instance boot พร้อมกันบนฐานข้อมูลใหม่ไม่ได้** `AutoMigrate` ถาม Postgres ว่ามี index อยู่หรือยัง
แล้วค่อยสร้าง — เป็น time-of-check-to-time-of-use race ล้วน ๆ พอสตาร์ตพร้อมกันทั้งสองโปรเซสผ่าน
การเช็คพร้อมกันแล้วสั่ง `CREATE` ทั้งคู่ ตัวหนึ่งตายด้วย
`duplicate key value violates unique constraint "pg_class_relname_nsp_index"` เกิดซ้ำ 3 จาก 3 ครั้ง
ตอนนี้ migration รันอยู่ใน transaction ที่ถือ `pg_advisory_xact_lock` instance จึงเข้าคิวแทนที่จะชนกัน
`pg_advisory_lock` ธรรมดาใช้ไม่ได้ตรงนี้ เพราะมันผูกกับ connection และคำสั่ง unlock อาจไปโดน
connection คนละตัวใน pool

---

## Metrics

`/metrics` เสิร์ฟ Prometheus text **ไม่มีการนับซ้ำที่ไหนเลย**: ฟังก์ชันเดียวแปลง metric families
เป็นตัวเลขบนหน้าจอ และมันถูกป้อนด้วย `prometheus.Gatherer` ของโปรเซสนี้ หรือด้วยหน้า metrics ของ
instance ที่ถูกเฝ้า — ดูหัวข้อ *Deployment roles* สองแหล่ง สูตรเดียว

histogram ของ redirect ใช้ bucket ที่เลือกเองตั้งแต่ 0.5 ms ขึ้นไป bucket ปริยายของ Prometheus
เริ่มที่ 5 ms ซึ่งจะทำให้ redirect ทุกครั้งกองอยู่ bucket แรกและ p99 อ่านไม่ได้ความ

ข้อจำกัดสองข้อที่ต้องระบุตรงนี้: **p99 เป็นค่าประมาณจากขอบ bucket** ไม่ใช่ค่าจริง และตัวเลข
**รีเซ็ตตอนรีสตาร์ต** นอกจากนี้ยังเป็นค่าต่อ instance — ดูหัวข้อ *รันมากกว่าหนึ่ง instance*

`/api/stats/public` เปิดเผย hit rate, p99 และจำนวนที่ถูก drop โดยไม่ต้องยืนยันตัวตน นี่เป็นการ
ตัดสินใจโดยเจตนาสำหรับโปรเจกต์ portfolio — ตัวเลขบนหน้า landing ตั้งใจให้ตรวจสอบได้ — และ
**ไม่ใช่สิ่งที่ควรลอกไปใช้**ในระบบที่ตัวเลขเชิงปฏิบัติการเป็นข้อมูลอ่อนไหว

---

## Logging

หนึ่งบรรทัด JSON ต่อหนึ่งคำขอ รวม `/:code` ด้วย:

```json
{"time":"...","level":"INFO","msg":"request","method":"GET","path":"/gopher","status":302,"duration_ms":1.9,"request_id":"56c6e84519ce77e0"}
```

ถ้าผู้เรียกส่ง `X-Request-ID` มาจะใช้ค่านั้นต่อ ตัวระบุที่ระบบต้นทางตั้งไว้จึงรอดมาถึงที่นี่ และมันถูก
ส่งกลับใน response ด้วย รายงานบั๊กจึงระบุคำขอที่ต้องการได้ตรงตัว ค่านี้ถูกจำกัดที่ 64 ตัวอักษร
เพราะเป็นค่าที่ผู้โจมตีควบคุมได้และไหลลงไปอยู่ใน log stream และมันถูกคัดลอกออกจาก buffer ของ
fasthttp ก่อนถูกเก็บ ด้วยเหตุผลเดียวกับ referrer — ดูหัวข้อ *บั๊กที่ test จับได้*

logger ของ GORM เองก็ถูกส่งผ่าน `slog` ด้วย ปิดสีและไม่นับ `ErrRecordNotFound` เป็น error
ก่อนหน้านั้น redirect ที่ 404 ทุกครั้งจะพ่นบล็อกหลายบรรทัดพร้อมรหัสสีออก stderr ซึ่งทำให้ข้อความ
ที่ว่า "log เป็น JSON" ไม่จริงในทางปฏิบัติ `LOG_FORMAT=text` (ค่าปริยาย) มีไว้อ่านใน terminal ส่วน
`LOG_FORMAT=json` มีไว้ให้อะไรก็ตามที่ทำ index จาก field

บรรทัด log ของ worker pool ไม่มี request id เพราะ click event ถูกรวมเป็น batch ข้ามคำขอ
ความสัมพันธ์หนึ่งต่อหนึ่งที่ request id สื่อถึงจึงไม่มีอยู่จริงตรงนั้น

access log ทำงานบนเส้นทาง redirect โดยไม่มีการ sample **ต้นทุนของมันยังไม่ได้ถูกวัด** จึงไม่มีการ
อ้างตัวเลขใด ๆ และนี่คือสิ่งแรกที่จะถูก sample ถ้ามันโผล่ขึ้นมาใน profile

---

## API

**สาธารณะ**

| Method | Path | |
|---|---|---|
| `POST` | `/api/links` | สร้างลิงก์ 20 req/min/IP |
| `GET` | `/api/links/:code/stats` | ยอดคลิก ซีรีส์ 14 วัน referrer 60 req/min/IP |
| `GET` | `/api/stats/public` | hit rate, p99, จำนวนที่ drop, ยอดคลิกรวม แบบสด |
| `GET` | `/:code` | 302 หรือ SPA shell พร้อม 404 จริง |
| `GET` | `/health` | liveness — `200` ตราบใดที่โปรเซสยังอยู่ ไม่เช็คอะไรเลย |
| `GET` | `/health/ready` | readiness — `503` ถ้าไม่มี Postgres, `200 degraded` ถ้าไม่มี Redis |
| `GET` | `/metrics` | |

**Admin** — JWT (HS256, 24 ชม.) อยู่ในคุกกี้ `httpOnly` แบบ `SameSite=Lax` **ไม่ใช่ `localStorage`**:
บั๊ก XSS ต้องอ่าน session ไม่ได้ ทุก route ที่เปลี่ยนแปลงข้อมูลเป็น `POST`/`DELETE` ซึ่ง `Lax`
กันข้ามไซต์ให้อยู่แล้ว จึงไม่ต้องมี CSRF token แยกต่างหาก

| Method | Path | |
|---|---|---|
| `POST` | `/api/admin/login`, `/api/admin/logout` | |
| `GET` | `/api/admin/me` | SPA อ่าน `exp` จากคุกกี้ httpOnly ไม่ได้ จึงต้องถาม |
| `GET` | `/api/admin/links` | คืนทุกแถว ฝั่ง client เรียง กรอง แบ่งหน้าเอง |
| `DELETE` | `/api/admin/links/:code` | ลบและ invalidate cache ทันที |
| `POST` | `/api/admin/links/:code/invalidate-cache` | |
| `GET` | `/api/admin/links/:code/analytics` | ซีรีส์ referrer เหตุการณ์ล่าสุด และ TTL ของ cache |
| `GET` | `/api/admin/dashboard/summary` | เกจสด poll ทุก 2 วิ |
| `GET` | `/api/admin/dashboard/overview` | aggregate หนัก cache 60 วิ poll ทุก 30 วิ |

แยกตามจังหวะรีเฟรชโดยเจตนา: การ poll ทุกสองวินาทีไม่ควรลาก `GROUP BY` ไปด้วยทุกครั้ง

IP ต้นทางถูก hash ด้วย SHA-256 ก่อนเข้า channel — ไม่มีเส้นทางโค้ดไหนถือ address ดิบไว้หลังจาก
handler จบ — และมีแค่ 8 ตัวอักษรแรกของ hash เท่านั้นที่ถูกส่งกลับไปหา client

---

## สิ่งที่โปรเจกต์นี้ไม่ได้เป็น

นี่คือโปรเจกต์ portfolio ไม่ใช่บริการ **มันไม่ถูกโฮสต์ที่ไหนโดยเจตนา**: URL shortener สาธารณะที่
ไม่มีการควบคุมการใช้งานในทางที่ผิดคือภาระ ไม่ใช่ของโชว์

นี่คือช่องว่างที่เหลือ พร้อมสิ่งที่ต้องจ่ายจริงถ้าจะปิดแต่ละข้อ:

**การเปลี่ยน schema ไม่มีเรื่องราวรองรับ** `AutoMigrate` รันตอน boot การ boot พร้อมกันปลอดภัยแล้ว
แต่ไม่มี versioned migration ไม่มี rollback และไม่มีอะไรที่จะทำให้การเปลี่ยน schema ลงได้ขณะมี
ทราฟฟิกจริง การปิดช่องนี้ต้องใช้เครื่องมือ migration, ตาราง migrations และวินัยแบบ expand/contract
กับการเปลี่ยนคอลัมน์ทุกครั้ง

**ไม่มีอะไรปลุกใคร** click event ที่ถูก drop ถูกนับและ log ไว้ และ `/metrics` ก็เปิดเผยมัน แต่ไม่มี
alert ไม่มี SLO ดังนั้น "เราทำ event หายไปหนึ่งชั่วโมง" จะมองเห็นได้เฉพาะคนที่บังเอิญไปเปิดดูเท่านั้น
การปิดช่องนี้ต้องใช้ Prometheus กับ Alertmanager, error budget ที่มีค่าพอจะปกป้อง และคนที่ถือ pager

**tracing หยุดอยู่ที่โปรเซสนี้** request id ผูกบรรทัด log ของโปรเซสเดียวเข้าด้วยกัน ไม่ไปไกลกว่านั้น
การปิดช่องนี้ต้องใช้ OpenTelemetry, collector และที่เก็บ trace

**ไม่มีการ deploy** ไม่มีปลายทาง ไม่มี artefact ที่มีเวอร์ชัน ไม่มี rollback — `docker compose up`
คือเรื่องราวการ deploy ทั้งหมดที่มี

**ลิงก์ไม่มีเจ้าของและสร้างแบบไม่ระบุตัวตน** ใครก็สร้างได้ ผู้ดูแลคนเดียวที่ถูก seed ไว้เห็นทุกอัน
และไม่มีอะไรตรวจว่าลิงก์ชี้ไปที่ไหน shortener จริงต้องมีการสแกน phishing และ malware ตอนสร้าง
มีช่องทางรายงานการใช้งานผิด และมีความเป็นเจ้าของรายบัญชี ก่อนจะเปิดให้คนทั่วไปใช้ได้
**นี่คือเหตุผลใหญ่ที่สุด**ที่โปรเจกต์นี้ไม่ได้เป็นบริการจริง

อีกสามข้อที่ควรพูดตรง ๆ เพราะมองข้ามได้ง่าย: p99 เป็นค่าประมาณจากขอบ bucket ไม่ใช่ค่าที่วัดตรง ๆ
ตัวเลขรีเซ็ตตอนรีสตาร์ต และ dashboard ที่ฝังมาแสดงผลของ instance เดียว
