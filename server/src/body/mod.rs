mod commands;
mod formatting;
mod fragment;
mod links;
pub mod persist_open;
mod urls;

pub use links::{cache_locations, post_location, KnownPostLocation};

use common::payloads::post_body::Node;

// TODO: unit tests
// TODO: newline handling tests

/// Flags post as open
const OPEN: u8 = 1;

/// Flags current fragment as quote
const QUOTED: u8 = 1 << 1;

const COUNTDOWN_PREFIX: &str = "countdown";
const AUTOBAHN_PREFIX: &str = "autobahn";

/// Parse post body into a Node tree. Different behavior for open and closed
/// posts.
///
/// All performed on one thread to maximize thread locality.
/// Yields of work sharing here are doubtable.
//
// TODO: finalization on post closure should be done with a separate async
// traversal function run by the Client
pub fn parse(body: &str, open: bool) -> Node {
	let mut dst = Node::Empty;
	if !body.is_empty() {
		let mut flags = 0;
		if open {
			flags |= OPEN;
		}
		formatting::parse_quoted(&mut dst, &body, flags);
	}
	dst
}

#[cfg(test)]
mod test {
	macro_rules! test_parsing {
		($(
			$name:ident(
				$in:expr => (
					$out_open:expr,
					$out_closed:expr$(,)?
				)
			)
		)+) => {
			$( mod $name {
				#![allow(unused_imports)]
				#![allow(unused)]
				use crate::body::*;
				use common::payloads::post_body::{
					Node::{self, *},
					PendingNode,
					EmbedProvider::*,
				};

				fn text(s: impl Into<String>) -> Node {
					Node::Text(s.into())
				}

				fn quote(inner: Node) -> Node {
					Node::Quoted(inner.into())
				}

				fn spoiler(inner: Node) -> Node {
					Node::Spoiler(inner.into())
				}

				fn code(s: impl Into<String>) -> Node {
					Node::Code(s.into())
				}

				macro_rules! gen_case {
					($fn_name:ident($open:literal, $out:expr)) => {
						#[test]
						fn $fn_name() {
							let mut conf = crate::config::Config::default();
							conf.public = {
								let mut p = common::config::Public::default();
								p.links = vec![(
									"4ch".to_owned(),
									"https://4channel.org".to_owned(),
								)]
									.into_iter()
									.collect();
								p.into()
							};
							crate::config::set(conf);

							links::cache_locations(
								std::iter::once(KnownPostLocation {
									id: 1,
									thread: 1,
									page: 0,
								}),
							);
							links::register_non_existent_post(3);

							let res = parse($in, $open);
							assert!(
								res == $out,
								"\ngot:      {:#?}\nexpected: {:#?}\n",
								res,
								$out,
							);
						}
					};
				}

				gen_case! { open(true, $out_open) }
				gen_case! { closed(false, $out_closed) }
			})+
		};
		($( $name:ident($in:expr => $out:expr) )+) => {
			test_parsing! {
				$(
					$name($in => ($out, $out))
				)+
			}
		};
	}

	/// Create a list of child nodes
	macro_rules! children {
		($($ch:expr),*$(,)?) => {
			Node::Children(vec![ $($ch,)* ])
		};
	}

	test_parsing! {
		simple("foo\nbar" => children![
			text("foo"),
			Newline,
			text("bar"),
		])
		quote(">foo\nbar" => children![
			quote(children![
				text(">foo"),
				Newline,
			]),
			text("bar"),
		])
		quote_with_multiple_gt(">>foo\nbar" => children![
			quote(children![text(">>foo"), Newline]),
			text("bar"),
		])
		spoiler("foo**bar** baz" => children![
			text("foo"),
			spoiler(text("bar")),
			text(" baz"),
		])
		multiline_spoiler("**foo\nbar**baz" => children![
			spoiler(children![
				text("foo"),
				Newline,
				text("bar"),
			]),
			text("baz"),
		])
		unclosed_spoiler_tags("**foo" => spoiler(text("foo")))
		unclosed_multiline_spoiler_tags("**foo\nbar" => spoiler(children![
			text("foo"),
			Newline,
			text("bar"),
		]))
		spoiler_in_quote(">baz **foo** bar" => quote(children![
			text(">baz "),
			spoiler(text("foo")),
			text(" bar"),
		]))
		spoiler_with_space("**foo **bar" => children![
			spoiler(text("foo ")),
			text("bar"),
		])
		post_link_right_after_quote(">>>1" => quote(children![
			text(">"),
			PostLink {
				id: 1,
				thread: 1,
				page: 0,
			},
		]))
		pending_post_link_right_after_quote(">>>2" => quote(children![
			text(">"),
			Pending(PendingNode::PostLink(2)),
		]))
		invalid_post_link_after_quote(">>>3" => quote(text(">>>3")))
		reference_right_after_quote(">>>>/4ch/" => quote(children![
			text(">"),
			Reference {
				label: "4ch".into(),
				url: "https://4channel.org".into(),
			},
		]))
		post_link_on_unquoted_line(">>1 a" => children![
			PostLink {
				id: 1,
				thread: 1,
				page: 0,
			},
			text(" a"),
		])
		reference_on_unquoted_line(">>>/4ch/ a" => children![
			Reference {
				label: "4ch".into(),
				url: "https://4channel.org".into(),
			},
			text(" a"),
		])
		spoiler_starting_in_line_middle_and_closing_on_the_next(
			"foo **bar\nbaz** woo" => children![
				text("foo "),
				spoiler(children![
					text("bar"),
					Newline,
					text("baz"),
				]),
				text(" woo"),
			]
		)
		spoiler_starting_in_line_middle_and_never_closing(
			"foo **bar\nbaz woo" => children![
				text("foo "),
				spoiler(children![
					text("bar"),
					Newline,
					text("baz woo"),
				]),
			]
		)
		spoiler_starting_in_quote_middle_and_closing_on_next(
			">foo **bar\n>baz** woo" => quote(children![
				text(">foo "),
				spoiler(children![
					text("bar"),
					Newline,
					text(">baz"),
				]),
				text(" woo"),
			])
		)
		spoiler_starting_in_quote_middle_and_never_closing(
			">foo **bar\n>baz woo" =>quote(children![
				text(">foo "),
				spoiler(children![
					text("bar"),
					Newline,
					text(">baz woo"),
				]),
			])
		)
		spoilers_on_multiple_quotation_levels(
			"**lol\n>foo **bar\n>baz woo\n>>EHHHHHHH" => children![
				spoiler(children![
					text("lol"),
					Newline,
				]),
				quote(children![
					text(">foo "),
					spoiler(children![
						text("bar"),
						Newline,
						text(">baz woo"),
						Newline,
					]),
				]),
				quote(text(">>EHHHHHHH")),
			]
		)
		multiline_bold_tags("foo @@bar\nbaz@@ foo" => children![
			text("foo "),
			Bold(
				children![
					text("bar"),
					Newline,
					text("baz"),
				]
				.into(),
			),
			text(" foo"),
		])
		multiline_italic_tags("foo ~~bar\nbaz~~ foo" => children![
			text("foo "),
			Italic(
				children![
					text("bar"),
					Newline,
					text("baz"),
				]
				.into(),
			),
			text(" foo"),
		])
		nested_overlapping_formatting("foo** bar@@b~~a@@zer**h" => children![
			text("foo"),
			spoiler(children![
				text(" bar"),
				Bold(
					children![
						text("b"),
						Italic(text("a").into()),
					]
					.into(),
				),
				text("zer"),
			]),
			text("h"),
		])
		explicitly_closed_formatting_of_entire_line("**foo**" => spoiler(
			text("foo"),
		))
		implicitly_closed_formatting_of_entire_line("**foo" => spoiler(
			text("foo"),
		))
		trailing_empty_line("foo\n" => children![text("foo"), Newline])
		edge_punctuation_leading(".#flip" => children![
			text("."),
			Pending(PendingNode::Flip),
		])
		edge_punctuation_trailing("#flip," => children![
			Pending(PendingNode::Flip),
			text(","),
		])
		edge_punctuation_both("(#flip," => children![
			text("("),
			Pending(PendingNode::Flip),
			text(","),
		])
		quoted_command(">#flip" => quote(text(">#flip")))
		flip("#flip" => Pending(PendingNode::Flip))
		eight_ball("#8ball" => Pending(PendingNode::EightBall))
		pyu("#pyu" => Pending(PendingNode::Pyu))
		pcount("#pcount" => Pending(PendingNode::PCount))
		countdown_explicit("#countdown(3)" => Pending(
			PendingNode::Countdown(3)
		))
		countdown_default("#countdown" => Pending(
			PendingNode::Countdown(10)
		))
		failed_command_with_trailing_parenthesis("#countdown_(3)" => text(
			"#countdown_(3)"
		))
		autobahn_explicit("#autobahn(3)" => Pending(
			PendingNode::Autobahn(3)
		))
		autobahn_default("#autobahn" => Pending(
			PendingNode::Autobahn(2)
		))
		code_explicit_language(r#"foo ``python print("bar")`` baz"# => children![
			text("foo "),
			code("<span class=\"syntex-source syntex-python\"><span class=\"syntex-meta syntex-function-call syntex-python\"><span class=\"syntex-meta syntex-qualified-name syntex-python\"><span class=\"syntex-support syntex-function syntex-builtin syntex-python\">print</span></span><span class=\"syntex-punctuation syntex-section syntex-arguments syntex-begin syntex-python\">(</span><span class=\"syntex-meta syntex-function-call syntex-arguments syntex-python\"><span class=\"syntex-meta syntex-string syntex-python\"><span class=\"syntex-string syntex-quoted syntex-double syntex-python\"><span class=\"syntex-punctuation syntex-definition syntex-string syntex-begin syntex-python\">&quot;</span></span></span><span class=\"syntex-meta syntex-string syntex-python\"><span class=\"syntex-string syntex-quoted syntex-double syntex-python\">bar<span class=\"syntex-punctuation syntex-definition syntex-string syntex-end syntex-python\">&quot;</span></span></span></span><span class=\"syntex-punctuation syntex-section syntex-arguments syntex-end syntex-python\">)</span></span></span>"),
			text(" baz"),
		])
		code_guessed_language("``#! /bin/bash\necho \"foo\"``" => code(
			"<span class=\"syntex-source syntex-shell syntex-bash\"><span class=\"syntex-comment syntex-line syntex-number-sign syntex-shell\"><span class=\"syntex-punctuation syntex-definition syntex-comment syntex-begin syntex-shell\">#</span></span><span class=\"syntex-comment syntex-line syntex-number-sign syntex-shell\">! /bin/bash</span><span class=\"syntex-comment syntex-line syntex-number-sign syntex-shell\"><br></span><span class=\"syntex-meta syntex-function-call syntex-shell\"><span class=\"syntex-support syntex-function syntex-echo syntex-shell\">echo</span></span><span class=\"syntex-meta syntex-function-call syntex-arguments syntex-shell\"> <span class=\"syntex-string syntex-quoted syntex-double syntex-shell\"><span class=\"syntex-punctuation syntex-definition syntex-string syntex-begin syntex-shell\">&quot;</span>foo<span class=\"syntex-punctuation syntex-definition syntex-string syntex-end syntex-shell\">&quot;</span></span></span></span>",
		))
		code_cant_guess_language("``foo()``" => code(
			"<span class=\"syntex-text syntex-plain\">foo()</span>",
		))
		code_invalid_explicit_language("``rash foo()``" => code(
			"<span class=\"syntex-text syntex-plain\">rash foo()</span>",
		))
		code_multiline("``bash echo $BAR\neval $BAZ" => code(
			"<span class=\"syntex-source syntex-shell syntex-bash\"><span class=\"syntex-meta syntex-function-call syntex-shell\"><span class=\"syntex-support syntex-function syntex-echo syntex-shell\">echo</span></span><span class=\"syntex-meta syntex-function-call syntex-arguments syntex-shell\"> <span class=\"syntex-meta syntex-group syntex-expansion syntex-parameter syntex-shell\"><span class=\"syntex-punctuation syntex-definition syntex-variable syntex-shell\">$</span><span class=\"syntex-variable syntex-other syntex-readwrite syntex-shell\">BAR</span></span></span><br><span class=\"syntex-meta syntex-function-call syntex-shell\"><span class=\"syntex-support syntex-function syntex-eval syntex-shell\">eval</span></span><span class=\"syntex-meta syntex-function-call syntex-arguments syntex-shell\"> <span class=\"syntex-meta syntex-group syntex-expansion syntex-parameter syntex-shell\"><span class=\"syntex-punctuation syntex-definition syntex-variable syntex-shell\">$</span><span class=\"syntex-variable syntex-other syntex-readwrite syntex-shell\">BAZ</span></span></span></span>"
		))
		code_multiline_trailing_newline("``bash echo $BAR\neval $BAZ\n``" => code(
			"<span class=\"syntex-source syntex-shell syntex-bash\"><span class=\"syntex-meta syntex-function-call syntex-shell\"><span class=\"syntex-support syntex-function syntex-echo syntex-shell\">echo</span></span><span class=\"syntex-meta syntex-function-call syntex-arguments syntex-shell\"> <span class=\"syntex-meta syntex-group syntex-expansion syntex-parameter syntex-shell\"><span class=\"syntex-punctuation syntex-definition syntex-variable syntex-shell\">$</span><span class=\"syntex-variable syntex-other syntex-readwrite syntex-shell\">BAR</span></span></span><br><span class=\"syntex-meta syntex-function-call syntex-shell\"><span class=\"syntex-support syntex-function syntex-eval syntex-shell\">eval</span></span><span class=\"syntex-meta syntex-function-call syntex-arguments syntex-shell\"> <span class=\"syntex-meta syntex-group syntex-expansion syntex-parameter syntex-shell\"><span class=\"syntex-punctuation syntex-definition syntex-variable syntex-shell\">$</span><span class=\"syntex-variable syntex-other syntex-readwrite syntex-shell\">BAZ</span></span></span><br></span>"
		))
		code_multiline_cross_line("foo ``bash echo $BAR\neval $BAZ`` null" => children![
			text("foo "),
			code("<span class=\"syntex-source syntex-shell syntex-bash\"><span class=\"syntex-meta syntex-function-call syntex-shell\"><span class=\"syntex-support syntex-function syntex-echo syntex-shell\">echo</span></span><span class=\"syntex-meta syntex-function-call syntex-arguments syntex-shell\"> <span class=\"syntex-meta syntex-group syntex-expansion syntex-parameter syntex-shell\"><span class=\"syntex-punctuation syntex-definition syntex-variable syntex-shell\">$</span><span class=\"syntex-variable syntex-other syntex-readwrite syntex-shell\">BAR</span></span></span><br><span class=\"syntex-meta syntex-function-call syntex-shell\"><span class=\"syntex-support syntex-function syntex-eval syntex-shell\">eval</span></span><span class=\"syntex-meta syntex-function-call syntex-arguments syntex-shell\"> <span class=\"syntex-meta syntex-group syntex-expansion syntex-parameter syntex-shell\"><span class=\"syntex-punctuation syntex-definition syntex-variable syntex-shell\">$</span><span class=\"syntex-variable syntex-other syntex-readwrite syntex-shell\">BAZ</span></span></span></span>"),
			text(" null"),
		])
		unknown_reference(">>>/fufufu/" => quote(text(">>>/fufufu/")))
		reference(">>>/4ch/" => Reference{
			label: "4ch".into(),
			url: "https://4channel.org".into(),
		})
		reference_with_extra_gt(">>>>/4ch/" => quote(children![
			text(">"),
			Reference{
				label: "4ch".into(),
				url: "https://4channel.org".into(),
			},
		]))
		reference_with_extra_gt_in_line_middle("f >>>>/4ch/" => children![
			text("f >"),
			Reference{
				label: "4ch".into(),
				url: "https://4channel.org".into(),
			},
		])
		invalid_reference_syntax(">>>/aaa" => quote(text(">>>/aaa")))
		invalid_post_link_syntax(">>3696+" => quote(text(">>3696+")))
		known_existing_post_link(">>1" => PostLink{
			id: 1,
			thread: 1,
			page: 0,
		})
		known_nonexisting_post_link(">>3" => quote(text(">>3")))
		unknown_post_link(">>2" => Pending(PendingNode::PostLink(2)))
		post_link_with_extra_gt(">>>1" => quote(children![
			text(">"),
			PostLink{
				id: 1,
				thread: 1,
				page: 0,
			},
		]))
		post_link_with_extra_ht_in_line_middle("f >>>>1" => children![
			text("f >>"),
			PostLink{
				id: 1,
				thread: 1,
				page: 0,
			},
		])
		empty_quote(">" => quote(text(">")))
		empty_double_quote(">>" => quote(text(">>")))
	}

	mod urls {
		macro_rules! test_valid {
			($( $name:ident($url:literal) )+) => {
				test_parsing! {
					$(
						$name($url => URL($url.into()))
					)+
				}
			};
		}

		test_valid! {
			http("http://foo.com")
			https("https://foo.com")
			ftp("ftp://foo.com")
			ftps("ftps://foo.com")
			magnet("magnet:?xt=urn:btih:61d887ca0bf1474d56a20950716fa443ce51ebdc&dn=%5BLewdas%5D%20Boku%20no%20Pico%20-%2001%20%5B346p%5D%5BHEVC%20444%20x265%2010bit%5D%5BEng-Subs%5D&tr=http%3A%2F%2Fsukebei.tracker.wf%3A8888%2Fannounce&tr=udp%3A%2F%2Fopen.stealth.si%3A80%2Fannounce&tr=udp%3A%2F%2Ftracker.opentrackr.org%3A1337%2Fannounce&tr=udp%3A%2F%2Fexodus.desync.com%3A6969%2Fannounce&tr=udp%3A%2F%2Ftracker.torrent.eu.org%3A451%2Fannounce")
			bitcoin("bitcoin:1F1tAaz5x1HUXrCNLbtMDqcw6o5GNn4xqX?amount=5.00?message=payment")
		}

		test_parsing! {
			leading_gt(">http://foo.com" => quote(children![
				text(">"),
				URL("http://foo.com".into()),
			]))
			invalid_url("http://><>.com" => text("http://><>.com"))
			unsupported_scheme("file:///aaaaaaaa" => text("file:///aaaaaaaa"))
		}
	}

	mod embeds {
		#![allow(non_snake_case)]

		macro_rules! test_embeds {
			($( $name:ident($url:literal => $prov:ident) )+) => {
				test_parsing! {
					$(
						$name($url => (
							URL($url.into()),
							Embed{
								provider: $prov,
								url: $url.into(),
							},
						))
					)+
				}
			};
		}

		test_embeds! {
			youtube_long("https://youtu.be/z0f4Wgi94eo" => YouTube)
			youtube_shortened("https://youtu.be/z0f4Wgi94eo" => YouTube)
			youtube_watch_query(
				"https://www.youtube.com/watch?v=z0f4Wgi94eo"
				=> YouTube
			)
			twitter(
				"https://twitter.com/siiteiebahiro/status/1379854507304124418?s=20"
				=> Twitter
			)
			imgur("https://imgur.com/gallery/YOy4UCA" => Imgur)
			soundcloud("https://soundcloud.com/cd_oblongar" => SoundCloud)
			coub("https://coub.com/view/2qc65u" => Coub)
			vimeo("https://vimeo.com/174312494" => Vimeo)
			bit_chute("https://www.bitchute.com/embed/z0f4Wgi94eo" => BitChute)
		}
	}

	mod dice {
		mod valid {
			macro_rules! test_valid {
				($(
					$name:ident($in:literal => {$rolls:literal $faces:literal})
				)+) => {
					$(
						mod $name {
							test_parsing! {
								no_offset(
									$in => 	Pending(PendingNode::Dice{
										offset: 0,
										faces: $faces,
										rolls: $rolls,
									})
								)
								plus_1(
									concat!($in, "+1") => 	Pending(
											PendingNode::Dice{
											offset: 1,
											faces: $faces,
											rolls: $rolls,
										}
									)
								)
								minus_1(
									concat!($in, "-1") => 	Pending(
										PendingNode::Dice{
											offset: -1,
											faces: $faces,
											rolls: $rolls,
										}
									)
								)
							}
						}
					)+
				};
			}

			test_valid! {
				implicit_single_die("#d10" => {1 10})
				explicit_single_die("#1d10" => {1 10})
				explicit_multiple_dice("#2d11" => {2 11})
			}
		}

		mod invalid {
			macro_rules! test_invalid {
				($( $name:ident($in:literal) )+) => {
					test_parsing! {
						$( $name($in => text($in)) )+
					}
				};
			}

			test_invalid! {
				// Dice parser is the final fallback for all unmatched commands
				invalid_command("#ass")
				not_dice("#dagger")

				too_many_dies("#11d6")
				too_many_faces("#d999999999999")
				too_big_offset("#d6+9999999999")
				too_small_offset("#d6-9999999999")
			}
		}
	}
}
