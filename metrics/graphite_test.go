package metrics

import (
	"bufio"
	"net"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

var (
	testQ = "Queue1"
	testG = "Group1"

	envGraphiteUDPAddr  = "WQS_GRAPHITE_UDP"
	envGraphiteHTTPAddr = "WQS_GRAPHITE_HTTP"
)

func readEnv() (udp, server string) {
	udp = os.Getenv(envGraphiteUDPAddr)
	if udp == "" {
		udp = "127.0.0.1:8333"
	}
	server = os.Getenv(envGraphiteHTTPAddr)
	if udp == "" {
		udp = "127.0.0.1"
	}
	return
}

func randData() []*metricsStat {
	var local = "localhost"
	return []*metricsStat{
		&metricsStat{
			Endpoint: local,
			Queue:    testQ,
			Group:    testG,
			Sent: &metricsStruct{
				Total:   100,
				Elapsed: 9.01,
				Scale:   map[string]int64{"less_10ms": 100},
			},
			Recv: &metricsStruct{
				Total:   100,
				Elapsed: 11.02,
				Latency: 100.92,
				Scale:   map[string]int64{"less_20ms": 100},
			},
			Accum: 0,
		},
	}
}

func TestGraphiteSend(t *testing.T) {
	var cnt uint64
	var udpServe = func(stop chan struct{}) error {
		laddr, err := net.ResolveUDPAddr("udp", ":10086")
		if err != nil {
			return err
		}
		l, err := net.ListenUDP("udp", laddr)
		if err != nil {
			return err
		}

		reader := bufio.NewReader(l)
		for {
			select {
			case <-stop:
				return l.Close()
			default:
			}
			line, err := reader.ReadSlice('\n')
			if err != nil {
				println("ERROR")
				continue
			}
			print(len(line), "bytes:", string(line))
			atomic.AddUint64(&cnt, 1)
		}
	}

	var metricsStructs = randData()

	stop := make(chan struct{})
	go udpServe(stop)
	cli := newGraphiteClient("localhost", "127.0.0.1:10086", "wqs")
	cli.Send("http://127.0.0.1:10086/upload", metricsStructs)
	time.Sleep(time.Second * 2)
	close(stop)
	if atomic.LoadUint64(&cnt) != 7 {
		t.FailNow()
	}
}

func TestGraphiteGroupMetrics(t *testing.T) {
	var testData = struct {
		start   int64
		end     int64
		step    int64
		group   string
		queue   string
		action  string
		metrics string
	}{
		start:   time.Now().Add(-1 * time.Hour).Unix(),
		end:     time.Now().Unix(),
		step:    1,
		group:   testG,
		queue:   testQ,
		action:  "sent",
		metrics: "qps",
	}

	udpAddr, reqAddr := readEnv()
	cli := newGraphiteClient(reqAddr, udpAddr, "wqs_local_test")
	data := randData()
	cli.Send("", data)
	time.Sleep(time.Second * 2)

	params := &MetricsQueryParam{
		Host:       AllHost,
		Queue:      testQ,
		Group:      testG,
		ActionKey:  KeySent,
		MetricsKey: KeyQps,
		StartTime:  testData.start,
		EndTime:    testData.end,
		Step:       testData.step,
	}
	ret, err := cli.GroupMetrics(params)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
	if ret == "" {
		t.Fatalf("got nil return from GroupMetrics")
	}
}
