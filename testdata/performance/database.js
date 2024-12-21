import http from 'k6/http';

/**
 * Modern databases tend to have latencies in the single-digit milliseconds.
 * We'll simulate 1-10ms latencies and 1-2 queries per request.
 */
export const options = {
    stages: [
        {duration: '20s', target: 100,},
        {duration: '20s', target: 200},
        {duration: '10s', target: 0},
    ],
    thresholds: {
        http_req_failed: ['rate<0.01'],
    },
};

// simulate different latencies
export default function () {
    // 1-10ms latency
    const latency = Math.floor(Math.random() * 9) + 1;
    // 1-2 iterations per request
    const iterations = Math.floor(Math.random() * 2) + 1;
    // 0-30000 work units per iteration
    const work = Math.floor(Math.random() *30000);
    // 0-40 output units
    const output = Math.floor(Math.random() * 40);

    http.get(http.url`${__ENV.CADDY_HOSTNAME}/sleep.php?sleep=${latency}&work=${work}&output=${output}&iterations=${iterations}`);
}