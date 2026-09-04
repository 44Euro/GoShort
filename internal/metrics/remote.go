package metrics

import (
	"context"
	"fmt"
	"net/http"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
)

// Remote อ่านหน้า /metrics ของอีก instance หนึ่งแล้วส่งเข้าสูตรคำนวณตัวเดียวกับ
// ที่โปรเซสนี้ใช้กับ registry ของตัวเอง
//
// ของจริงงานนี้เป็นหน้าที่ของ Prometheus ที่ scrape ทุก instance แล้วรวมให้
// การดึงตรงแบบ hop เดียวนี้มีไว้ให้คอนโซลเฝ้า instance เดียวได้โดยไม่ต้องยก
// Prometheus ขึ้นมาทั้งชุด ไม่ใช่ทางที่ควรทำตามเมื่อมี instance มากกว่านี้
type Remote struct {
	URL    string
	Client *http.Client
}

// timeout ต้องสั้นกว่าจังหวะ poll ของคอนโซล ไม่งั้น instance ที่ค้างจะลากหน้าจอค้างตาม
const remoteTimeout = 1500 * time.Millisecond

func NewRemote(url string) Remote {
	return Remote{URL: url, Client: &http.Client{Timeout: remoteTimeout}}
}

func (r Remote) Summary(ctx context.Context) (Summary, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.URL, nil)
	if err != nil {
		return Summary{}, err
	}

	res, err := r.Client.Do(req)
	if err != nil {
		return Summary{}, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return Summary{}, fmt.Errorf("metrics source answered %d", res.StatusCode)
	}

	// TextParser แบบ zero-value panic ตอน parse เพราะ scheme ปริยายเป็น unset
	// ต้องผ่าน constructor เท่านั้น
	parser := expfmt.NewTextParser(model.UTF8Validation)
	byName, err := parser.TextToMetricFamilies(res.Body)
	if err != nil {
		return Summary{}, err
	}

	families := make([]*dto.MetricFamily, 0, len(byName))
	for _, f := range byName {
		families = append(families, f)
	}
	return SummaryFrom(families), nil
}
