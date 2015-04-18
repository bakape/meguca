var _ = require('underscore'),
	async = require('async'),
	config = require('../config'),
	crypto = require('crypto'),
	fs = require('fs'),
	hooks = require('../hooks'),
	imager = require('../imager/config'),
	lang = require('../lang'),
	options = require('../alpha/options/common'),
	path = require('path'),
	report = require('../report/config'),
	vm = require('vm');

_.templateSettings = {
	interpolate: /\{\{(.+?)\}\}/g
};

exports.emitter = new (require('events').EventEmitter);

exports.dbCache = {
	OPs: {},
	opTags: {},
	threadSubs: {},
	YAKUMAN: 0,
	funThread: 0,
	addresses: {},
	ranges: {},
};

var HOT = exports.hot = {};
var RES = exports.resources = {};
exports.clientConfig = [];
exports.clientConfigHash = '';
exports.clients = {};
exports.clientsByIP = {};

function reload_hot_config(cb) {
	fs.readFile('hot.js', 'UTF-8', function (err, js) {
		if (err)
			cb(err);
		var hot = {};
		try {
			vm.runInNewContext(js, hot);
		}
		catch (e) {
			return cb(e);
		}
		if (!hot || !hot.hot)
			return cb('Bad hot config.');

		// Overwrite the original object just in case
		Object.keys(HOT).forEach(function (k) {
			delete HOT[k];
		});
		_.extend(HOT, hot.hot);

		// Pass some of the config variables to the client
		var clientHot = _.pick(HOT,
			'ILLYA_DANCE',
			'EIGHT_BALL',
			'THREADS_PER_PAGE',
			'ABBREVIATED_REPLIES',
			'SUBJECT_MAX_LENGTH',
			'EXCLUDE_REGEXP',
			'ADMIN_ALIAS',
			'MOD_ALIAS',
			'SAGE_ENABLED',
			'THREAD_LAST_N'
		);

		reloadCSS(clientHot, function(err) {
			if (err)
				return cb(err);
			HOT.CLIENT_CONFIG = JSON.stringify(clientConfig);
			HOT.CLIENT_IMAGER = JSON.stringify(clientImager);
			HOT.CLIENT_REPORT = JSON.stringify(clientReport);
			HOT.CLIENT_HOT = JSON.stringify(clientHot);
			var combined = exports.clientConfig = [
				clientConfig,
				clientImager,
				clientReport,
				clientHot
			];
			exports.clientConfigHash = HOT.CLIENT_CONFIG_HASH = crypto
				.createHash('MD5')
				.update(JSON.stringify(combined))
				.digest('hex');

			read_exits('exits.txt', function() {
				hooks.trigger('reloadHot', HOT, cb);
			});
		});
	});
}

var clientConfig = _.pick(config,
	'IP_MNEMONIC',
	'USE_WEBSOCKETS',
	'SOCKET_PATH',
	'DEBUG',
	'READ_ONLY',
	'API_URL',
	'IP_TAGGING',
	'RADIO',
	'PYU',
	'BOARDS',
	'LANGS',
	'DEFAULT_LANG'
);
var clientImager = _.pick(imager,
	'WEBM',
	'UPLOAD_URL',
	'MEDIA_URL',
	'THUMB_DIMENSIONS',
	'PINKY_DIMENSIONS',
	'SPOILER_IMAGES',
	'IMAGE_HATS',
	'ASSETS_DIR',
	'BANNERS'
);
var clientReport = _.pick(report, 'RECAPTCHA_PUBLIC_KEY');

function reload_scripts(cb) {
	async.mapSeries(['client', 'vendor', 'mod'], getRevision,
		function(err, js) {
			if (err)
				return cb(err);
			HOT.CLIENT_JS = js[0].client;
			HOT.VENDOR_JS = js[1].vendor;
			// Read moderator js file
			fs.readFile(path.join('state', js[2].mod), 'UTF-8',
				function (err, modSrc) {
					if (err)
						return cb(err);
					RES.modJs = modSrc;
					cb(null);
				}
			);
		}
	);
}

// TEMP: Non-DRY for now. Seperated for the new client.
function reload_alpha_client(cb) {
	var stream = fs.createReadStream('./www/js/alpha.js'),
		hash = crypto.createHash('md5');
	stream.once('error', function(err) {
		cb(err);
	});
	stream.on('data', function(data) {
		hash.update(data);
	});
	stream.once('end', function() {
		HOT.ALPHA_HASH = hash.digest('hex').slice(0, 8);
		cb(null);
	});
}

// Read JSON files in ./state, generated by grunt-rev
function getRevision(name, cb) {
	fs.readFile(path.join('state', name + '.json'), function(err, json) {
		if (err)
			return cb(err);
		var files;
		try {
			files = JSON.parse(json);
		}
		catch(e) {
			return cb(e);
		}
		if (!files)
			return cb('Bad state/' + name + '.json');
		cb(null, files);
	});
}

function reloadCSS(hot, cb) {
	getRevision('css', function(err, files) {
		if (err)
			return cb(err);
		// Only the curfew template is statically assigned a CSS file. The rest
		// are inserted per request in server/render.js and www/js/setup.js
		HOT.CURFEW_CSS = files['curfew.css'];
		// Export to these modules and client
		HOT.css = hot.css = files;
		cb(null);
	});
}

function reload_resources(cb) {
	read_templates(function (err, tmpls) {
		if (err)
			return cb(err);

		_.extend(RES, expand_templates(tmpls));

		hooks.trigger('reloadResources', RES, cb);
	});
}

function read_templates(cb) {
	function read(dir, file) {
		return fs.readFile.bind(fs, path.join(dir, file), 'UTF-8');
	}

	async.parallel({
		index: read('tmpl', 'index.html'),
		alpha: read('tmpl', 'alpha.html'),
		filter: read('tmpl', 'filter.html'),
		login: read('tmpl', 'login.html'),
		curfew: read('tmpl', 'curfew.html'),
		suspension: read('tmpl', 'suspension.html'),
		aLookup: read('tmpl', 'alookup.html'),
		notFound: read('www', '404.html'),
		serverError: read('www', '50x.html'),
	}, cb);
}

function expand_templates(res) {
	var templateVars = _.clone(HOT);
	_.extend(templateVars, imager, config, make_navigation_html());

	templateVars.SCHEDULE = build_schedule(templateVars.SCHEDULE);
	templateVars.FAQ = build_FAQ(templateVars.FAQ);
	// Format info banner
	if (templateVars.BANNERINFO)
		templateVars.BANNERINFO = `&nbsp;&nbsp;[${templateVars.BANNERINFO}]`;

	// Insert variables into the templates
	function tmpl(data, vars) {
		var expanded = _.template(data)(vars ||templateVars);
		return {tmpl: expanded.split(/\$[A-Z]+/),
			src: expanded};
	}

	var ex = {
		filterTmpl: tmpl(res.filter).tmpl,
		curfewTmpl: tmpl(res.curfew).tmpl,
		suspensionTmpl: tmpl(res.suspension).tmpl,
		loginTmpl: tmpl(res.login).tmpl,
		aLookupHtml: res.aLookup,
		notFoundHtml: res.notFound,
		serverErrorHtml: res.serverError,
	};

	// Build index templates for each language
	config.LANGS.forEach(function(ln) {
		var html, hash;
		// Inject the localised variables
		_.extend(templateVars, lang[ln].tmpl);
		// Build localised options panel
		templateVars.options_panel = buildOptions(lang[ln].opts);
		templateVars.lang = JSON.stringify(lang[ln].common);
		html = tmpl(res.alpha);
		ex['alphaTmpl-' + ln] = html.tmpl;
		hash = crypto.createHash('md5').update(html.src);
		ex['alphaHash-' + ln] = hash.digest('hex').slice(0, 8)
	});

	// Legacy client template
	var html, hash;
	html = tmpl(res.index);
	ex.indexTmpl = html.tmpl;
	hash = crypto.createHash('md5').update(html.src);
	ex.indexHash = hash.digest('hex').slice(0, 8);

	return ex;
}

function build_schedule(schedule){
	var filler = ['drink & fap', 'fap & drink', 'tea & keiki'];
	var table = ['<table>'];
	for (var day in schedule){
		var plans = schedule[day].plans;
		var time = schedule[day].time;
		// Fill empty slots
		if (plans == '')
			plans = filler[Math.floor(Math.random() * filler.length)];
		if (time == '')
			time = 'all day';
		table.push('<tr><td><b>[', day + ']&nbsp;&nbsp;', '</b></td><td>',
				plans + '&nbsp;&nbsp;', '</td><td>', time, '</td></tr>');
	}
	table.push('</table>');
	return table.join('');
}

function build_FAQ(faq){
	if (faq.length > 0){
		var list = ['<ul>'];
		faq.forEach(function(entry){
			list.push('<li>' + entry + '</li>');
		});
		list.push('<ul>');
		return list.join('');
	}
}

// Hardcore pornography
function buildOptions(lang) {
	var html = '<div class="bmodal" id="options-panel">'
		+ '<ul class="option_tab_sel">';
	;
	lang.tabs.forEach(function(tab, index) {
		html += `<li><a data-content="tab-${index}"`;
		// Highlight the first tabButt by default
		if (index === 0)
			html += ' class="tab_sel"';
		html += `>${tab}</a></li>`;
	});
	html += '</ul><ul class="option_tab_cont">';
	lang.tabs.forEach(function(tab, index) {
		var opts = _.filter(options, function(opt) {
			/*
			 * Pick the options for this specific tab. Don't know why we have
			 * undefineds inside the array, but we do.
			 */
			if (!opt || opt.tab != index)
				return false;
			// Option should not be loaded, because of server-side configs
			return !(opt.load !== undefined && !opt.load);
		});
		html += `<li class="tab-${index}`;
		// Show the first tab by default
		if (index == 0)
			html += ' tab_sel';
		html += '">';
		// Render the actual options
		opts.forEach(function(opt) {
			const isShortcut = opt.type == 'shortcut',
				isList = opt.type instanceof Array,
				isCheckbox = opt.type == 'checkbox' || opt.type === undefined,
				isNumber = opt.type == 'number',
				isImage = opt.type == 'image';
			if (isShortcut)
				html += 'Alt+';
			if (!isList) {
				html += '<input';
				if (isCheckbox || isImage)
					html += ` type="${(isCheckbox ? 'checkbox' : 'file')}"`;
				if (isNumber)
					html += ' style="width: 4em;" maxlength="4"';
				else if (isShortcut)
					html += ' maxlength="1"';
			}
			else
				html += '<select';
			// Custom localisation functions
			var title, tooltip;
			if (opt.lang) {
				title = lang[opt.lang][1](opt.id);
				label = lang[opt.lang][0](opt.id);
			}
			else {
				title = lang[opt.id][1];
				label = lang[opt.id][0];
			}
			html += ` id="${opt.id}" title="${title}">`;

			if (isList) {
				opt.type.forEach(function(item) {
					html += `<option value="${item}">${lang[item] || item}</option>`;
				});
				html += '</select>';
			}
			html += `<label for="${opt.id}" title="${title}">${label}</label><br>`;
		});
		// Append Export and Import links to first tab
		if (index == 0) {
			html += '<br>';
			['export', 'import'].forEach(function(id) {
				html += `<a id="${id}" title="${lang[id][1]}">${lang[id][0]}</a> `;
			});
			// Hidden file input for uploading the JSON
			html += '<input type="file" style="display: none;" id="importSettings"'
				+ ' name="Import Settings"></input>';
		}
		html += '</li>';
	});
	html += '</ul></div>';
	return html;
}

exports.reload_hot_resources = function (cb) {
	async.series([
		reload_hot_config,
		reload_scripts,
		reload_alpha_client,
		reload_resources,
	], cb);
};

function make_navigation_html() {
	if (!HOT.INTER_BOARD_NAVIGATION)
		return '';
	var bits = '<b id="navTop">[';
	// Actual boards
	config.BOARDS.forEach(function (board, i) {
		if (board == config.STAFF_BOARD)
			return;
		if (i > 0)
			bits += ' / ';
		bits += `<a href="../${board}/" class="history">${board}</a>`;
	});
	// Add custom URLs to board navigation
	config.PSUEDO_BOARDS.forEach(function(item) {
		bits += ` / <a href="${item[1]}">${item[0]}</a>`;
	});
	bits += ']</b>';
	return {NAVTOP: bits};
}

function read_exits(file, cb) {
	fs.readFile(file, 'UTF-8', function (err, lines) {
		if (err)
			return cb(err);
		var dest = HOT.BANS;
		lines.split(/\n/g).forEach(function (line) {
			var m = line.match(/^(?:^#\d)*(\d+\.\d+\.\d+\.\d+)/);
			if (!m)
				return;
			var exit = m[1];
			if (dest.indexOf(exit) < 0)
				dest.push(exit);
		});
		cb(null);
	});
}
