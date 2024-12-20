import http from 'k6/http';
import { sleep } from 'k6';

const ioLatencyMilliseconds = 0;
const workIterations = 0;
const outputIterations = 1;

export const options = {
    stages: [
        { duration: '5s', target: 100, },
        { duration: '20s', target: 200 },
        { duration: '20s', target: 0 },
    ],
    thresholds: {
        http_req_failed: ['rate<0.01'],
        http_req_duration: ['p(90)<3'],
    },
};

export default function () {
    http.get(`${__ENV.CADDY_HOSTNAME}/sleep.php?sleep=${ioLatencyMilliseconds}&work=${workIterations}&output=${outputIterations}`);
    //sleep(1);
}