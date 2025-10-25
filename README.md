# golang-webapp

This project provides a basic web application [skeleton](https://wiki.c2.com/?WalkingSkeleton) written in golang to make it easier to get something rolling without trying to figure out how to link all the bits together.

Features:

* Static assets served directly from the webapp.
* Server-side-rendered HTML templates, with buffered template rendering to prevent output of incomplete or malformed content in the event of template evaluation errors.
* Two build modes:
  * `dev` - serves static assets and templates directly from the local filesystem. Allows for development of templates and static assets without needing to restart the application.
  * `prod` - embeds templates and static assets into the application binary to allow ease of distribution. Changes to templates or static assets require a new application build.
