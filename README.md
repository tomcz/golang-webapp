# golang-webapp

This project provides a basic web application [skeleton](https://wiki.c2.com/?WalkingSkeleton) written in golang, to make it easier to get something rolling without trying to figure out how to link all the bits together. Instead of relying on third-party http routing and rendering frameworks this web application attempts to stick to packages provided by the golang sdk itself. Third-party packages are used to provide low-level security middleware and session storage/caching backends.

Features:

* Cookie-based, server memory-based, or memcached-based HTTP sessions.
* Static assets served directly from the webapp.
* Server-side-rendered HTML templates, with buffered template rendering to prevent output of incomplete or malformed content in the event of template evaluation errors.
* `dev` build to serve static assets and templates directly from the local filesystem, allowing for development of templates and static assets without needing to restart the webapp.
* `prod` build that embeds templates and static assets into the application binary to allow ease of distribution.
