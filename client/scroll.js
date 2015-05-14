/*
 * Various page scrolling logic
 */

var $ = require('jquery'),
    main = require('./main'),
    PAGE_BOTTOM = -1,
    nestLevel = 0,
    state = require('./state'),
    thread = state.page.get('thread');
    
var lockTarget, lockKeyHeight;
var $lockTarget, $lockIndicator;
var lockedManually;
    
// Checks if we're at the bottom of page at the moment    
var at_bottom = function() {
	return window.scrollY + window.innerHeight >= main.$doc.height() - 5;
};
if (window.scrollMaxY !== undefined)
	at_bottom = function () {
		return window.scrollMaxY <= window.scrollY;
	};

// Sets the scroll lock position (to a post or to bottom of window)
function set_lock_target(num, manually) {
	lockedManually = manually;
	if (!num && at_bottom())
		num = PAGE_BOTTOM;
	if (num == lockTarget)
		return;
	lockTarget = num;
        console.log('locktarget set to: '+lockTarget);
	var bottom = lockTarget == PAGE_BOTTOM;
	if ($lockTarget)
		$lockTarget.removeClass('scroll-lock');
	if (num && !bottom && manually)
		$lockTarget = $('#' + num).addClass('scroll-lock');
	else
		$lockTarget = null;

	var $ind = $lockIndicator;
	if ($ind) {
		var visible = bottom || manually;
		$ind.css({visibility: visible ? 'visible' : 'hidden'});
		if (bottom)
			$ind.text('Locked to bottom');
		else if (num) {
			$ind.empty().append($('<a/>', {
				text: '>>' + num,
				href: '#' + num,
			}));
		}
	}
}

/* 
 * Logic for locking position to bottom of thread
 * Records the original scroll position before function is called
 * Adjusts the scroll position back to original after function executes.
 * Use for every action that would change length of a thread.
 */
function followLock(func) {
	var lockHeight, locked = lockTarget, $post;
	if (locked == PAGE_BOTTOM)
		lockHeight = main.$doc.height();
	else if (locked) {
		$post = $('#' + locked);
		var r = $post.length && $post[0].getBoundingClientRect();
		if (r && r.bottom > 0 && r.top < window.innerHeight)
			lockHeight = r.top;
		else
			locked = false;
	}

	var ret;
	try {
		nestLevel++;
		ret = func.call(this);
	}
	finally {
		if (!--nestLevel)
			Backbone.trigger('flushDomUpdates');
//  This won't work since we don't have this in yet.
//  And I don't know why it's important so I'll get it in later
//  Quality quality control at its finest s(' ^)b
	}

	if (locked == PAGE_BOTTOM) {
		var height = main.$doc.height();
		if (height > lockHeight - 10)
			window.scrollBy(0, height - lockHeight + 10);
	}
	else if (locked && lockTarget == locked) {
		var newY = $post[0].getBoundingClientRect().top;
		window.scrollBy(0, newY - lockHeight);
	}

	return ret;
}
exports.followLock = followLock;

(function () {
        /* Uncomment when certain of menuHandler things being functional
         * Locks to post
	menuHandlers.Focus = function (model) {
		var num = model && model.id;
		set_lock_target(num, true);
	};
        //Unlocks from post or bottom
	menuHandlers.Unfocus = function () {
		set_lock_target(null);
	};
        */
       
	//Check if user scrolled to the bottom every time they scroll
        function scroll_shita() {
		if (!lockTarget || (lockTarget == PAGE_BOTTOM))
			set_lock_target(null);
	}

	if (thread) {
		$lockIndicator = $('<span id=lock>Locked to bottom</span>', {
			css: {visibility: 'hidden'},
		}).appendTo('body');
		main.$doc.scroll(scroll_shita);
		scroll_shita();
	}
})();

// If a post is a locked target and becomes hidden, unlock from post.

Backbone.on('hide', function (model) {
	if (model && model.id == lockTarget)
		set_lock_target(null);
});

// Account for banner height, when scrolling to an anchor

function aboveBanner (){
	if (!/^#\d+$/.test(location.hash))
		return;
	$(window).scrollTop($(location.hash).offset().top - $('#banner').height());
}
exports.aboveBanner = aboveBanner;

window.onload = exports.aboveBanner;
