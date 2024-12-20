import http from 'k6/http';
import { sleep } from 'k6';

const ioLatencyMilliseconds = 15000;
const workIterations = 100;
const outputIterations = 1;

export const options = {
    stages: [
        { duration: '20s', target: 50, },   // ramp up to concurrency 10 over 20s
        { duration: '20s', target: 300 },    // ramp up to concurrency 25 over 20s
        { duration: '20s', target: 0 },     // ramp down to 0 over 20s
    ],
    thresholds: {
        http_req_failed: ['rate<0.01'],     // http errors should be less than 1%
        http_req_duration: ['p(90)<1100'],   // 90% of requests should be below 150ms
    },
};

export default function () {
    // 1 hanging request
    http.get(`${__ENV.CADDY_HOSTNAME}/sleep.php?sleep=${ioLatencyMilliseconds}&work=${workIterations}&output=${outputIterations}`);

    // 5 regular requests
    for (let i = 0; i < 5; i++) {
        http.get(`${__ENV.CADDY_HOSTNAME}/sleep.php?sleep`);
    }
}