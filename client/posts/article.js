/*
 * Reply posts
 */

let main = require('../main'),
	postCommon = require('./common'),
	{_, Backbone, options, state} = main;

var Article = module.exports = Backbone.View.extend({
	tagName: 'article',
	render() {
		this.setElement(main.oneeSama.article(this.model.attributes));
		return this;
	},
	insertIntoDOM() {
		main.$threads.children('#p' + this.model.get('op'))
			.children('blockquote, .omit, form, article[id]:last')
			.last()
			.after(this.$el);
		this.autoExpandImage().fun();
	}
});

// Extend with common mixins
_.extend(Article.prototype, postCommon);
