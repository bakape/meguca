// Code generated by qtc from "index.html". DO NOT EDIT.
// See https://github.com/valyala/quicktemplate for details.

//line index.html:1
package templates

//line index.html:1
import "github.com/bakape/meguca/config"

//  TODO: Default language

//line index.html:1
import (
	qtio422016 "io"

	qt422016 "github.com/valyala/quicktemplate"
)

//line index.html:1
var (
	_ = qtio422016.Copy
	_ = qt422016.AcquireByteBuffer
)

//line index.html:1
func StreamMain(qw422016 *qt422016.Writer, c config.Configs) {
//line index.html:1
	qw422016.N().S(`<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><meta name="application-name" content="meguca"><meta name="description" content="Realtime imageboard"><link type="image/x-icon" rel="shortcut icon" id="favicon" href="/assets/favicons/default.ico"><title id="page-title">meguca</title><link rel="manifest" href="/assets/mobile/manifest.json"><link rel="stylesheet" href="/assets/css/base.css" type="text/css"><link rel="stylesheet" id="theme-css" href="/assets/css/`)
//line index.html:13
	qw422016.N().S(c.DefaultCSS)
//line index.html:13
	qw422016.N().S(`.css" type="text/css"><style id="user-background-style"></style><script>if (localStorage.theme&& localStorage.theme !== "`)
//line index.html:17
	qw422016.N().S(c.DefaultCSS)
//line index.html:17
	qw422016.N().S(`") {document.getElementById('theme-css').href =`)
//line index.html:17
	qw422016.N().S("`")
//line index.html:17
	qw422016.N().S(`/assets/css/${localStorage.theme}.css`)
//line index.html:17
	qw422016.N().S("`")
//line index.html:17
	qw422016.N().S(`;}window.language_pack = new Promise((resolve, reject) => {fetch(`)
//line index.html:17
	qw422016.N().S("`")
//line index.html:17
	qw422016.N().S(`/assets/lang/${localStorage.lang || "en_GB"}.json`)
//line index.html:17
	qw422016.N().S("`")
//line index.html:17
	qw422016.N().S(`).then(r => r.text()).then(resolve).catch(reject)});</script></head><body><div id="user-background"></div><div class="overlay-container"><span id="banner" class="glass"><b id="banner-center" class="spaced"></b><span><b id="sync" class="banner-float svg-link" lang-title="sync"></b><b id="sync-counter" class="act hide-empty banner-float svg-link" lang-title="sync_count"></b><b id="thread-post-counters" class="act hide-empty banner-float svg-link" lang-title="posts_images"></b><span id="banner-extensions" class="hide-empty banner-float svg-link"></span><a id="banner-FAQ" class="banner-float svg-link" lang-title="faq"><svg xmlns="http://www.w3.org/2000/svg" width="8" height="8" viewBox="0 0 8 8"><path d="M3 0c-.55 0-1 .45-1 1s.45 1 1 1 1-.45 1-1-.45-1-1-1zm-1.5 2.5c-.83 0-1.5.67-1.5 1.5h1c0-.28.22-.5.5-.5s.5.22.5.5-1 1.64-1 2.5c0 .86.67 1.5 1.5 1.5s1.5-.67 1.5-1.5h-1c0 .28-.22.5-.5.5s-.5-.22-.5-.5c0-.36 1-1.84 1-2.5 0-.81-.67-1.5-1.5-1.5z" transform="translate(2)" /></svg></a><a id="banner-identity" class="banner-float svg-link" lang-title="identity"><svg xmlns="http://www.w3.org/2000/svg" width="8" height="8" viewBox="0 0 8 8"><path d="M4 0c-1.1 0-2 1.12-2 2.5s.9 2.5 2 2.5 2-1.12 2-2.5-.9-2.5-2-2.5zm-2.09 5c-1.06.05-1.91.92-1.91 2v1h8v-1c0-1.08-.84-1.95-1.91-2-.54.61-1.28 1-2.09 1-.81 0-1.55-.39-2.09-1z" /></svg></a><a id="banner-options" class="banner-float svg-link" lang-title="options"><svg xmlns="http://www.w3.org/2000/svg" width="8" height="8" viewBox="0 0 8 8"><path d="M3.5 0l-.5 1.19c-.1.03-.19.08-.28.13l-1.19-.5-.72.72.5 1.19c-.05.1-.09.18-.13.28l-1.19.5v1l1.19.5c.04.1.08.18.13.28l-.5 1.19.72.72 1.19-.5c.09.04.18.09.28.13l.5 1.19h1l.5-1.19c.09-.04.19-.08.28-.13l1.19.5.72-.72-.5-1.19c.04-.09.09-.19.13-.28l1.19-.5v-1l-1.19-.5c-.03-.09-.08-.19-.13-.28l.5-1.19-.72-.72-1.19.5c-.09-.04-.19-.09-.28-.13l-.5-1.19h-1zm.5 2.5c.83 0 1.5.67 1.5 1.5s-.67 1.5-1.5 1.5-1.5-.67-1.5-1.5.67-1.5 1.5-1.5z" /></svg></a></span></span><div id="modal-overlay" class="overlay"></div></div><div class="overlay top-overlay" id="hover-overlay"></div><div id="captcha-overlay" class="overlay top-overlay"></div><noscript><div style="width: 100%; height: auto; position: absolute; top: 40%; text-align: center;"><b style="font-size: 15vw;">FUCK OFF</b></div></noscript><section id="threads"></section><script src="/assets/client/index.js"></script></body></html>`)
//line index.html:70
}

//line index.html:70
func WriteMain(qq422016 qtio422016.Writer, c config.Configs) {
//line index.html:70
	qw422016 := qt422016.AcquireWriter(qq422016)
//line index.html:70
	StreamMain(qw422016, c)
//line index.html:70
	qt422016.ReleaseWriter(qw422016)
//line index.html:70
}

//line index.html:70
func Main(c config.Configs) string {
//line index.html:70
	qb422016 := qt422016.AcquireByteBuffer()
//line index.html:70
	WriteMain(qb422016, c)
//line index.html:70
	qs422016 := string(qb422016.B)
//line index.html:70
	qt422016.ReleaseByteBuffer(qb422016)
//line index.html:70
	return qs422016
//line index.html:70
}