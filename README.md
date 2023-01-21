# Localbooru

Localbooru is a booru that implements a subset of Danbooru's API and is
intended to be used by Boorumux to save images from any booru.

## Warning

Please do not read the code.

I wanted to get this going in one night and as a result it looks pretty
disgusting, so I plan to rewrite a good portion of it eventually.
Then again, most temporary solutions are permanent.

## Dependencies

- SQLite3 and a C compiler for the database
- ImageMagick for thumbnails

## Setup

Localbooru is meant to intertwine with
[Boorumux](https://github.com/KushBlazingJudah/boorumux) so setup is generally
easy.

You will want to use the `localbooru` branch of Boorumux in order to gain
support.

In `boorumux.json`, set `"local": "http://127.0.0.1:8081"` and then add a new
booru of type Danbooru called "local", with the attribute `"proxy": "none!"`.

When you start Boorumux now, you should see the new "local" booru and whenever
you browse images from boorus, you can click "Save to Localbooru" on the
sidebar to place them in Localbooru.
When the page finishes loading, Localbooru has saved the image and hopefully
Boorumux redirected you back to the page you were on.

You may view all of your saved images in the "local" booru, complete with tag
and rating search.
