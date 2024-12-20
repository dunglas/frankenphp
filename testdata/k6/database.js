import http from 'k6/http';
import { sleep } from 'k6';

/**
 * Modern databases tend to have latencies in the single-digit milliseconds.
 */
export const options = {
    stages: [
        { duration: '20s', target: 50, },   // ramp up to concurrency 10 over 20s
        { duration: '20s', target: 200 },    // ramp up to concurrency 25 over 20s
        { duration: '20s', target: 0 },     // ramp down to 0 over 20s
    ],
    thresholds: {
        http_req_failed: ['rate<0.01'],     // http errors should be less than 1%
        http_req_duration: ['p(90)<5'],   // 90% of requests should be below 150ms
    },
};

// simulate different latencies
export default function () {
    http.get(`${__ENV.CADDY_HOSTNAME}/sleep.php?sleep=1work=5000&output=10`);
    http.get(`${__ENV.CADDY_HOSTNAME}/sleep.php?sleep=5work=5000&output=10`);
    http.get(`${__ENV.CADDY_HOSTNAME}/sleep.php?sleep=10work=5000&output=10`);
}