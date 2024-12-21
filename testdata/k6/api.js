import http from 'k6/http';

/**
 * Many applications communicate with external APIs or microservices.
 * Latencies tend to be much higher than with databases in these cases.
 * We'll consider 10ms-150ms
 */
export const options = {
    stages: [
        { duration: '20s', target: 150, },
        { duration: '20s', target: 400 },
        { duration: '10s', target: 0 },
    ],
    thresholds: {
        http_req_failed: ['rate<0.01'],
    },
};

// simulate different latencies
export default function () {
    // 10-150ms latency
    const latency = Math.floor(Math.random() * 140) + 10;
    // 0-30000 work units
    const work = Math.floor(Math.random() * 30000);
    // 0-40 output units
    const output = Math.floor(Math.random() * 40);

    http.get(http.url`${__ENV.CADDY_HOSTNAME}/sleep.php?sleep=${latency}&work=${work}&output=${output}`);
}