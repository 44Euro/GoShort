import http from "k6/http";
import { check } from "k6";

const BASE = __ENV.BASE_URL || "http://localhost:8099";
const CODES = (__ENV.CODES || "gopher").split(",");

// ต้องขอ p(99) เอง k6 ไม่ใส่มาให้ใน summary export โดยปริยาย
export const summaryTrendStats = ["avg", "med", "p(95)", "p(99)", "max"];

export const options = {
  scenarios: {
    redirects: {
      executor: "constant-vus",
      vus: Number(__ENV.VUS || 100),
      duration: __ENV.DURATION || "30s",
    },
  },
  thresholds: {
    // ตั้งไว้กว้าง ๆ เพราะสคริปต์นี้ใช้ "วัด" ไม่ใช่ "ตัดสินผ่าน/ไม่ผ่าน"
    http_req_failed: ["rate<0.01"],
  },
};

export default function () {
  const code = CODES[Math.floor(Math.random() * CODES.length)];
  const res = http.get(`${BASE}/${code}`, { redirects: 0 });
  check(res, { "302": (r) => r.status === 302 });
}
