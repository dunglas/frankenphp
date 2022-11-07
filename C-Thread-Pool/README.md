[![CircleCI](https://circleci.com/gh/Pithikos/C-Thread-Pool.svg?style=svg)](https://circleci.com/gh/Pithikos/C-Thread-Pool)

# C Thread Pool

This is a minimal but advanced threadpool implementation.

  * ANCI C and POSIX compliant
  * Pause/resume/wait as you like
  * Simple easy-to-digest API
  * Well tested

The threadpool is under MIT license. Notice that this project took a considerable amount of work and sacrifice of my free time and the reason I give it for free (even for commercial use) is so when you become rich and wealthy you don't forget about us open-source creatures of the night. Cheers!

If this project reduced your development time feel free to buy me a coffee.

[![Donate](https://www.paypal.com/en_US/i/btn/x-click-but21.gif)](https://www.paypal.me/seferidis)


## Run an example

The library is not precompiled so you have to compile it with your project. The thread pool
uses POSIX threads so if you compile with gcc on Linux you have to use the flag `-pthread` like this:

    gcc example.c thpool.c -D THPOOL_DEBUG -pthread -o example


Then run the executable like this:

    ./example


## Basic usage

1. Include the header in your source file: `#include "thpool.h"`
2. Create a thread pool with number of threads you want: `threadpool thpool = thpool_init(4);`
3. Add work to the pool: `thpool_add_work(thpool, (void*)function_p, (void*)arg_p);`

The workers(threads) will start their work automatically as fast as there is new work
in the pool. If you want to wait for all added work to be finished before continuing
you can use `thpool_wait(thpool);`. If you want to destroy the pool you can use
`thpool_destroy(thpool);`.


## API

For a deeper look into the documentation check in the [thpool.h](https://github.com/Pithikos/C-Thread-Pool/blob/master/thpool.h) file. Below is a fast practical overview.

| Function example                | Description                                                         |
|---------------------------------|---------------------------------------------------------------------|
| ***thpool_init(4)***            | Will return a new threadpool with `4` threads.                        |
| ***thpool_add_work(thpool, (void&#42;)function_p, (void&#42;)arg_p)*** | Will add new work to the pool. Work is simply a function. You can pass a single argument to the function if you wish. If not, `NULL` should be passed. |
| ***thpool_wait(thpool)***       | Will wait for all jobs (both in queue and currently running) to finish. |
| ***thpool_destroy(thpool)***    | This will destroy the threadpool. If jobs are currently being executed, then it will wait for them to finish. |
| ***thpool_pause(thpool)***      | All threads in the threadpool will pause no matter if they are idle or executing work. |
| ***thpool_resume(thpool)***      | If the threadpool is paused, then all threads will resume from where they were.   |
| ***thpool_num_threads_working(thpool)***  | Will return the number of currently working threads.   |


## Contribution

You are very welcome to contribute. If you have a new feature in mind, you can always open an issue on github describing it so you don't end up doing a lot of work that might not be eventually merged. Generally we are very open to contributions as long as they follow the below keypoints.

* Try to keep the API as minimal as possible. That means if a feature or fix can be implemented without affecting the existing API but requires more development time, then we will opt to sacrifice development time.
* Solutions need to be POSIX compliant. The thread-pool is advertised as such so it makes sense that it actually is.
* For coding style simply try to stick to the conventions you find in the existing codebase.
* Tests: A new fix or feature should be covered by tests. If the existing tests are not sufficient, we expect an according test to follow with the pull request.
* Documentation: for a new feature please add documentation. For an API change the documentation has to be thorough and super easy to understand.

If you wish to **get access as a collaborator** feel free to mention it in the issue https://github.com/Pithikos/C-Thread-Pool/issues/78
