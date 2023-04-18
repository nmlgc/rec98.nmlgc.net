# Licenses

(Written by [Nmlgc](https://github.com/nmlgc).)

## [CC0 1.0]

Applies to the text content in the blog, the databases, and any files embedded
in the blog that do not contain elements copyrighted by third parties:

* `db/*.tsv`
* `blog/*.html`
* `blog/2019-08*`
* `blog/2019-09-04*`
* `blog/2021-09-12*`
* `static/favicon.ico`
* `static/faq-pi-ranges.svg`
* `static/logo.png`

### Rationale

I did consider adding the [Attribution clause]; after all, keeping the community
aware of this project is a big component of its ongoing success. But then I
noticed that the actual things that people are likely to copy-paste from the
blog are screenshots and videos of copyrighted games, which in turn makes these
screenshots and videos copyrighted and not licensable.\
That only leaves the text content on the blog. Due to its technical nature, it's
way more likely to be shared in paraphrased or summarized form, which bypasses
the need for attribution anyway.

Also, the blog posts mainly discuss observable facts about a third-party game.
In that light, it's hard to argue that alternative descriptions of these facts,
released later, constitute a derivative work of a corresponding ReC98 blog post.
For an example, see the [alternate explanation of TH01's unused and glitch
stages], posted to *The Cutting Room Floor* a few days after ReC98 covered the
latter in the [2022-08-14 blog post](https://rec98.nmlgc.net/blog/2022-08-14).\
Therefore, putting the entire blog text into the public domain is much less of a
headache for everyone. The community has always been attributing this project as
a whole even without a license, and will likely continue to do so.

## [GNU Affero General Public License v3.0]

Applies to all code written for the website:

* `.gitignore`
* `*.css`
* `*.go`
* `*.html` (only on the root directory of this repository)
* `*.ts`
* `go.mod`
* `go.sum`

### Rationale

Chosen mainly to keep open the possibility of integrating code from DOSBox-X
(which itself is GPL'd), like the ZMBV codec or full-blown PC-98 emulation, on
the website itself. I've also been sympathetic to the GPL for a long time, and
especially lately after seeing how companies typically treat open-source
software.\
If you want to reuse parts of this codebase under a different license, feel free
to contact me, and we might work something out.

## [GNU General Public License v2.0]

* `blog/2020-09-03-tup-bac02a5-win32.zip`
  (archive contains a link to the source code repository)

## [GitHub logo usage terms]

* `static/github.svg`

## [Twitter Trademark Guidelines]

* `static/twitter.svg`

## [SIL Open Font License, Version 1.1]

* `static/video.woff2` (a derivative of [Catrinity](https://catrinity-font.de))

## [Stripe's Mark Usage Terms]

* `static/emoji-stripe.svg`

## No license

All other files not covered by any of the licenses above. These include

* screenshots, videos, and mod downloads embedded in the blog
* game icons and custom emoji
* meme images with unclear copyright holders

----

[Attribution clause]: https://creativecommons.org/licenses/by/4.0/
[CC0 1.0]: https://creativecommons.org/publicdomain/zero/1.0/
[GNU General Public License v2.0]: https://www.gnu.org/licenses/old-licenses/gpl-2.0
[GNU Affero General Public License v3.0]: https://www.gnu.org/licenses/agpl-3.0.html
[GitHub logo usage terms]: https://github.com/logos
[Twitter Trademark Guidelines]: https://about.twitter.com/en/who-we-are/brand-toolkit
[alternate explanation of TH01's unused and glitch stages]: https://tcrf.net/index.php?title=Touhou_Reiiden:_The_Highly_Responsive_to_Prayers&oldid=1286863#Unused_Levels
[SIL Open Font License, Version 1.1]: https://scripts.sil.org/cms/scripts/page.php?item_id=OFL_web
[Stripe's Mark Usage Terms]: https://stripe.com/legal/marks
