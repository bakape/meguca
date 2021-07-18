package db

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bakape/meguca/auth"
	"github.com/bakape/meguca/common"
	"github.com/bakape/meguca/config"
	"github.com/bakape/meguca/static"
	"github.com/bakape/meguca/util"
	"github.com/go-playground/log"
	"github.com/lib/pq"
)

var version = len(migrations)

var migrations = []func(tx *sql.Tx) error{
	func(tx *sql.Tx) (err error) {
		// Initialize DB
		err = execAll(tx,
			`create table accounts (
				id varchar(20) primary key,
				password bytea not null
			)`,
			`create table sessions (
				account varchar(20) not null references accounts on delete cascade,
				token text not null,
				expires timestamp not null,
				primary key (account, token)
			)`,
			`create table bans (
				board varchar(3) not null,
				ip inet not null,
				by varchar(20) not null,
				reason text not null,
				expires timestamp default now(),
				primary key (ip, board)
			)`,
			`create table images (
				apng boolean not null,
				audio boolean not null,
				video boolean not null,
				fileType smallint not null,
				thumbType smallint not null,
				dims smallint[4] not null,
				length int not null,
				size int not null,
				md5 char(22) not null,
				sha1 char(40) primary key
			)`,
			`create table image_tokens (
				token char(86) not null primary key,
				sha1 char(40) not null references images on delete cascade,
				expires timestamp not null
			)`,
			`create table boards (
				readOnly boolean not null,
				textOnly boolean not null,
				forcedAnon boolean not null,
				hashCommands boolean not null,
				codeTags boolean not null,
				id varchar(3) primary key,
				ctr bigint default 0,
				created timestamp not null,
				title varchar(100) not null,
				notice varchar(500) not null,
				rules varchar(5000) not null,
				eightball text[] not null
			)`,
			`create table staff (
				board varchar(3) not null references boards on delete cascade,
				account varchar(20) not null references accounts on delete cascade,
				position varchar(50) not null
			)`,
			`create index staff_board on staff (board)`,
			`create index staff_account on staff (account)`,
			`create sequence post_id`,
			`create table threads (
				locked boolean,
				board varchar(3) not null references boards on delete cascade,
				id bigint primary key,
				postCtr bigint not null,
				imageCtr bigint not null,
				bumpTime bigint not null,
				replyTime bigint not null,
				subject varchar(100) not null,
				log text[] not null
			)`,
			`create index threads_board on threads (board)`,
			`create index bumpTime on threads (bumpTime)`,
			`create table posts (
				editing boolean not null,
				spoiler boolean,
				deleted boolean,
				banned boolean,
				id bigint primary key,
				op bigint not null references threads on delete cascade,
				time bigint not null,
				board varchar(3) not null,
				trip char(10),
				auth varchar(20),
				sha1 char(40) references images on delete set null,
				name varchar(50),
				imageName varchar(200),
				body varchar(2000) not null,
				password bytea,
				ip inet,
				links bigint[][2],
				backlinks bigint[][2],
				commands json[]
			)`,
			`create index deleted on posts (deleted)`,
			`create index op on posts (op)`,
			`create index image on posts (sha1)`,
			`create index editing on posts (editing)`,
			`create index ip on posts (ip)`,
		)
		if err != nil {
			return
		}

		data, err := json.Marshal(config.Defaults)
		if err != nil {
			return
		}
		_, err = sq.Insert("main").
			Columns("id", "val").
			Values("config", string(data)).
			RunWith(tx).
			Exec()
		if err != nil {
			return
		}

		err = CreateAdminAccount(tx)
		if err != nil {
			return
		}
		return CreateSystemAccount(tx)
	},
	func(tx *sql.Tx) (err error) {
		// Delete legacy columns
		return execAll(tx,
			`ALTER TABLE threads
				DROP COLUMN locked`,
			`ALTER TABLE boards
				DROP COLUMN hashCommands,
				DROP COLUMN codeTags`,
		)
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`ALTER TABLE threads
				DROP COLUMN log`,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`ALTER TABLE boards
				DROP COLUMN ctr`,
		)
		return
	},
	// Restore correct image counters after incorrect updates
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`UPDATE threads
				SET imageCtr = (SELECT COUNT(*) FROM posts
					WHERE SHA1 IS NOT NULL
						AND op = threads.id
				)`,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`ALTER TABLE images
				ADD COLUMN Title varchar(100) not null default '',
				ADD COLUMN Artist varchar(100) not null default ''`,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`ALTER TABLE posts
				ADD COLUMN sage bool`,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(`DROP INDEX deleted`)
		return
	},
	// Set default expiry configs, to keep all threads from deleting
	func(tx *sql.Tx) (err error) {
		return patchConfigsLegacy(tx, func(conf *config.Configs) {
			conf.ThreadExpiryMin = config.Defaults.ThreadExpiryMin
			conf.ThreadExpiryMax = config.Defaults.ThreadExpiryMax
		})
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`ALTER TABLE boards
				ADD COLUMN disableRobots bool default false`,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`ALTER TABLE threads
				ADD COLUMN sticky bool default false`,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`ALTER TABLE bans
				ADD COLUMN forPost bigint default 0`,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		return execAll(tx,
			`create table mod_log (
				type smallint not null,
				board varchar(3) not null,
				id bigint not null,
				by varchar(20) not null,
				created timestamp default (now() at time zone 'utc')
			)`,
			`create index mod_log_board on mod_log (board)`,
			`create index mod_log_created on mod_log (created)`,
		)
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(`create index sticky on threads (sticky)`)
		return
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`ALTER TABLE posts
				DROP COLUMN backlinks`,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`create table banners (
				board varchar(3) not null references boards on delete cascade,
				id smallint not null,
				data bytea not null,
				mime text not null
			);`,
		)
		return
	},
	func(tx *sql.Tx) error {
		return execAll(tx,
			`alter table boards
				alter column id type text`,
			`alter table bans
				alter column board type text`,
			`alter table mod_log
				alter column board type text`,
			`alter table staff
				alter column board type text`,
			`alter table banners
				alter column board type text`,
			`alter table threads
				alter column board type text`,
			`alter table posts
				alter column board type text`,
		)
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`create table loading_animations (
				board text primary key references boards on delete cascade,
				data bytea not null,
				mime text not null
			);`,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`ALTER TABLE boards
				ADD COLUMN defaultCSS text default 'moe'`,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`ALTER TABLE posts
				ADD COLUMN flag char(2)`,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`ALTER TABLE boards
				ADD COLUMN flags bool default false`,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`alter table images
				alter column title type varchar(200)`,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`ALTER TABLE boards
				ADD COLUMN NSFW bool default false`,
		)
		return
	},
	func(tx *sql.Tx) error {
		return execAll(tx,
			`create table reports (
				id bigserial primary key,
				target bigint not null,
				board text not null,
				reason text not null,
				by inet not null,
				illegal boolean not null,
				created timestamp default (now() at time zone 'utc')
			)`,
			`create index report_board on reports (board)`,
			`create index report_created on reports (created)`,
		)
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`ALTER TABLE boards
				ADD COLUMN nonLive bool default false`,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`ALTER TABLE threads
				ADD COLUMN nonLive bool default false`,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`ALTER TABLE boards
				ADD COLUMN posterIDs bool default false`,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`ALTER TABLE posts
				ADD COLUMN posterID text`,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`ALTER TABLE threads
				ADD COLUMN locked bool default false`,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`ALTER TABLE boards
				ADD COLUMN js varchar(5000) default ''`,
		)
		return
	},
	// Fix consequences of bug in init.sql
	func(tx *sql.Tx) (err error) {
		q := `SELECT EXISTS (SELECT 1
			FROM information_schema.columns
			WHERE table_schema='public'
				AND table_name='posts'
				AND column_name='locked'
		)`
		var exists bool
		err = tx.QueryRow(q).Scan(&exists)
		if err != nil || !exists {
			return
		}

		// Correct it
		_, err = tx.Exec(
			`ALTER TABLE posts
				DROP COLUMN locked`,
		)
		if err != nil {
			return
		}
		_, err = tx.Exec(
			`ALTER TABLE threads
				ADD COLUMN locked bool default false`,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`ALTER TABLE boards
				DROP COLUMN js`,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		return execAll(tx,
			`DROP INDEX editing`,
			`ALTER TABLE boards
				DROP COLUMN nonLive`,
			`ALTER TABLE posts
				DROP COLUMN editing`,
		)
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`ALTER TABLE posts
				DROP COLUMN password`,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		return execAll(tx,
			`ALTER TABLE boards
				ADD COLUMN nonLive bool default false`,
			`ALTER TABLE posts
				ADD COLUMN 	editing boolean not null default false`,
			`create index editing on posts (editing);`,
		)
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`ALTER TABLE posts
				ADD COLUMN password bytea`,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`INSERT INTO main VALUES ('roulette', '6')`,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		// Fuck any wise guy trying to create an account nad block an upgrade
		_, err = tx.Exec(
			`DELETE FROM accounts
			WHERE id = 'system'`,
		)
		if err != nil {
			return
		}

		password, err := auth.RandomID(32)
		if err != nil {
			return
		}
		hash, err := auth.BcryptHash(password, 10)
		if err != nil {
			return
		}
		_, err = tx.Exec(
			`insert into accounts (id, password)
			values ('system', $1)`,
			hash,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		return patchConfigsLegacy(tx, func(conf *config.Configs) {
			conf.CharScore = config.Defaults.CharScore
			conf.PostCreationScore = config.Defaults.PostCreationScore
			conf.ImageScore = config.Defaults.ImageScore
		})
	},
	func(tx *sql.Tx) (err error) {
		var rcount string
		err = tx.QueryRow(
			`SELECT COUNT(*) FROM mod_log WHERE board != 'all' AND by = 'system' AND type = 0`,
		).Scan(&rcount)
		if err != nil {
			return
		}
		_, err = tx.Exec(
			`INSERT INTO main VALUES ('rcount', $1)`,
			rcount,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		execAll(tx,
			`create table links (
				source bigint not null references posts on delete cascade,
				target bigint not null references posts on delete cascade,
				primary key(source, target)
			);`,
			`create index link_source on links (source);`,
			`create index link_target on links (target);`,
		)
		if err != nil {
			return
		}

		// Read all posts and links
		r, err := tx.Query(
			`select p.id, p.op, p.links from posts as p
			join threads as t on t.id = p.op`,
		)
		if err != nil {
			return
		}
		defer r.Close()
		var (
			posts  = make(map[uint64]bool, 1<<10)
			links  = make(map[uint64]uint64, 1<<10)
			id, op uint64
			lr     linkRowLegacy
		)
		for r.Next() {
			err = r.Scan(&id, &op, &lr)
			if err != nil {
				return
			}
			posts[id] = true
			for _, pair := range lr {
				links[id] = pair[0]
			}
		}
		if err != nil {
			return
		}

		// Remove legacy link row
		_, err = tx.Exec(`alter table posts drop column links`)
		if err != nil {
			return
		}

		// Write only verified links to new table
		q, err := tx.Prepare(
			`insert into links (source, target)
			values ($1, $2)`,
		)
		if err != nil {
			return
		}
		for source, target := range links {
			if !posts[source] || !posts[target] {
				continue
			}
			_, err = q.Exec(source, target)
			if err != nil {
				return
			}
		}

		return
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(`ALTER TABLE banners DROP COLUMN id`)
		return
	},
	func(tx *sql.Tx) error {
		return execAll(tx,
			`DELETE FROM mod_log`,
			`DELETE FROM bans`,
			`ALTER TABLE mod_log ADD CONSTRAINT mod_log_board_fkey
			FOREIGN KEY (board) REFERENCES boards(id) ON DELETE CASCADE`,
			`ALTER TABLE bans ADD CONSTRAINT bans_board_fkey
			FOREIGN KEY (board) REFERENCES boards(id) ON DELETE CASCADE`,
		)
	},
	func(tx *sql.Tx) error {
		return execAll(tx,
			`ALTER TABLE threads DROP COLUMN postCtr`,
			`ALTER TABLE threads DROP COLUMN imageCtr`,
		)
	},
	func(tx *sql.Tx) error {
		return execAll(tx,
			`ALTER TABLE mod_log ADD COLUMN length bigint default 0`,
			`ALTER TABLE mod_log ADD COLUMN reason text default ''`,
		)
	},
	func(tx *sql.Tx) error {
		return execAll(tx,
			`create index image_fileType on images (fileType)`,
			`create index image_audio on images (audio)`,
			`create index post_board on posts (board)`,
		)
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`ALTER TABLE boards
				ADD COLUMN rbText bool default false`,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		// Read all commands
		var (
			comms = make(map[uint64][]common.Command, 1024)
			id    uint64
		)
		err = queryAll(
			sq.Select("id", "commands").
				From("posts").
				Where("commands is not null").
				RunWith(tx),
			func(r *sql.Rows) (err error) {
				var com commandRow
				err = r.Scan(&id, &com)
				if err != nil {
					return
				}
				comms[id] = []common.Command(com)
				return
			},
		)
		if err != nil {
			return
		}

		prep, err := tx.Prepare(
			`update posts
			set commands = $2
			where id = $1`)
		if err != nil {
			return
		}

		// Remove all #pyu/#pcount commands
		new := make(commandRow, 0, 64)
		for id, comms := range comms {
			new = new[:0]
			for _, c := range comms {
				switch c.Type {
				case common.Pyu, common.Pcount:
				default:
					new = append(new, c)
				}
			}
			val := new
			if len(new) == 0 {
				val = nil
			}
			_, err = prep.Exec(id, val)
			if err != nil {
				return
			}
		}

		return
	},
	func(tx *sql.Tx) error {
		_, err := sq.Delete("main").Where("id = 'pyu'").RunWith(tx).Exec()
		return err
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`ALTER TABLE boards
				ADD COLUMN pyu bool default false`,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`create table pyu (
				id text primary key references boards on delete cascade,
				pcount bigint default 0
			);
			create table pyu_limit (
				ip inet not null,
				board text not null references boards on delete cascade,
				expires timestamp not null,
				pcount smallint default 4,
				primary key(ip, board)
			);
			create index pyu_limit_ip on pyu_limit (ip);
			create index pyu_limit_board on pyu_limit (board);`,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		// Reverted migration
		return
	},
	func(tx *sql.Tx) (err error) {
		// Revert changes
		return execAll(tx,
			`alter table boards drop column pyu`,
			`drop table pyu`,
			`drop table pyu_limit`,
		)
	},
	func(tx *sql.Tx) (err error) {
		return patchConfigsLegacy(tx, func(conf *config.Configs) {
			conf.EmailErrMail = config.Defaults.EmailErrMail
			conf.EmailErrPass = config.Defaults.EmailErrPass
			conf.EmailErrSub = config.Defaults.EmailErrSub
			conf.EmailErrPort = config.Defaults.EmailErrPort
		})
	},
	// Fixes global moderation
	func(tx *sql.Tx) (err error) {
		c := BoardConfigs{
			BoardConfigs: config.AllBoardConfigs.BoardConfigs,
			Created:      time.Now().UTC(),
		}

		_, err = sq.Insert("boards").
			Columns(
				"id", "readOnly", "textOnly", "forcedAnon", "disableRobots",
				"flags", "NSFW",
				"rbText", "created", "defaultCSS", "title",
				"notice", "rules", "eightball").
			Values(
				c.ID, c.ReadOnly, c.TextOnly, c.ForcedAnon, c.DisableRobots,
				c.Flags, c.NSFW, c.RbText,
				c.Created, c.DefaultCSS, c.Title, c.Notice, c.Rules,
				pq.StringArray(c.Eightball)).
			RunWith(tx).
			Exec()
		if err != nil {
			return
		}

		// Legacy function
		writeStaff := func(tx *sql.Tx, board string,
			staff map[string][]string,
		) (err error) {
			// Remove previous staff entries
			_, err = sq.Delete("staff").
				Where("board  = ?", board).
				RunWith(tx).
				Exec()
			if err != nil {
				return
			}

			// Write new ones
			q, err := tx.Prepare(`insert into staff (board, account, position)
				values($1, $2, $3)`)
			if err != nil {
				return
			}
			for pos, accounts := range staff {
				for _, a := range accounts {
					_, err = q.Exec(board, a, pos)
					if err != nil {
						return
					}
				}
			}

			return
		}

		return writeStaff(tx, "all", map[string][]string{
			"owners": {"admin", "system"},
		})
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`ALTER TABLE boards
				ADD COLUMN pyu bool default false`,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`create table pyu (
				id text primary key references boards on delete cascade,
				pcount bigint default 0
			);
			create table pyu_limit (
				ip inet not null,
				board text not null references boards on delete cascade,
				expires timestamp not null,
				pcount smallint default 4,
				primary key(ip, board)
			);
			create index pyu_limit_ip on pyu_limit (ip);
			create index pyu_limit_board on pyu_limit (board);`,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		r, err := tx.Query(`select id from boards`)
		if err != nil {
			return
		}
		defer r.Close()

		var boards []string
		for r.Next() {
			var board string
			err = r.Scan(&board)
			if err != nil {
				return
			}
			boards = append(boards, board)
		}
		err = r.Err()
		if err != nil {
			return
		}

		q, err := tx.Prepare(`insert into pyu (id, pcount) values ($1, 0)`)
		if err != nil {
			return
		}
		for _, b := range boards {
			_, err = q.Exec(b)
			if err != nil {
				return
			}
		}

		return
	},
	func(tx *sql.Tx) (err error) {
		return execAll(tx,
			`alter table pyu_limit drop column expires`,
			`alter table pyu_limit add column restricted bool default false`,
		)
	},
	func(tx *sql.Tx) (err error) {
		return execAll(tx,
			`create table captchas (
				id text primary key not null,
				solution text not null,
				expires timestamp not null
			)`,
			`create table failed_captchas (
				ip inet not null,
				expires timestamp not null
			)`,
		)
	},
	func(tx *sql.Tx) (err error) {
		return execAll(tx,
			`create index failed_captchas_ip on failed_captchas (ip)`,
		)
	},
	func(tx *sql.Tx) (err error) {
		var tasks []string
		for _, t := range [...]string{
			"image_tokens", "bans", "captchas", "failed_captchas",
		} {
			tasks = append(tasks, createIndex(t, "expires"))
		}
		tasks = append(tasks, createIndex("posts", "time"))
		return execAll(tx, tasks...)
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`INSERT INTO main VALUES ('geo_md5', 'initial value, ignore')`,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		_, err = sq.Delete("main").Where("id = 'roulette'").RunWith(tx).Exec()
		if err != nil {
			return
		}
		_, err = sq.Delete("main").Where("id = 'rcount'").RunWith(tx).Exec()
		return
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`create table roulette (
				id bigint primary key references threads on delete cascade,
				scount smallint default 6,
				rcount smallint default 0
			);
			create index roulette_rcount on roulette (rcount);`,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		var threads []uint64
		r, err := tx.Query(`select id from threads`)

		if err != nil {
			return
		}

		defer r.Close()

		for r.Next() {
			var thread uint64
			err = r.Scan(&thread)

			if err != nil {
				return
			}

			threads = append(threads, thread)
		}

		err = r.Err()

		if err != nil {
			return
		}

		q, err := tx.Prepare(
			`insert into roulette (id, scount, rcount) values ($1, 6, 0)`)

		if err != nil {
			return
		}

		for _, t := range threads {
			_, err = q.Exec(t)

			if err != nil {
				return
			}
		}

		return
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`create table spam_scores (
				ip inet primary key,
				score bigint not null
			);`,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		return execAll(tx,
			`alter table boards drop column nonLive`,
			`alter table threads drop column nonLive`,
		)
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`alter table posts add column meidoVision bool default false`,
		)
		return
	},
	func(tx *sql.Tx) (err error) {
		var tasks []string

		for _, col := range [...]string{"deleted", "banned", "meidovision"} {
			tasks = append(tasks, "alter table posts drop column "+col)
		}
		tasks = append(
			tasks,
			`alter table posts add column moderated bool not null default false`,
			`create table post_moderation (
				post_id bigint not null references posts on delete cascade,
				type smallint not null,
				by text not null,
				length bigint not null,
				reason text not null
			)`,
			createIndex("post_moderation", "post_id"),
		)

		return execAll(tx, tasks...)
	},
	func(tx *sql.Tx) (err error) {
		var tasks []string
		setNotNull := func(col, typ, def string) {
			tasks = append(tasks, fmt.Sprintf(
				`UPDATE posts
				SET %s = %s
				WHERE %s IS NULL`,
				col, def, col,
			))
			for _, s := range [...]string{
				"SET DATA TYPE " + typ,
				"SET DEFAULT " + def,
				"SET NOT NULL",
			} {
				tasks = append(tasks, fmt.Sprintf(
					`ALTER TABLE posts ALTER COLUMN %s %s`, col, s))
			}
		}

		for _, col := range [...]string{"spoiler", "sage"} {
			setNotNull(col, "bool", "false")
		}
		for _, col := range [...]string{
			"name", "trip", "auth", "imageName", "flag", "posterID",
		} {
			setNotNull(col, "text", `''::text`)
		}

		return execAll(tx, tasks...)
	},
	func(tx *sql.Tx) (err error) {
		for _, t := range [...]string{"mod_log", "post_moderation"} {
			_, err = tx.Exec(
				fmt.Sprintf(`alter table %s rename column reason to data`, t))
			if err != nil {
				return
			}
		}
		return
	},
	func(tx *sql.Tx) (err error) {
		return execAll(tx,
			`alter table bans drop constraint bans_pkey`,
			createIndex("bans", "ip"),
			createIndex("bans", "board"),
		)
	},
	func(tx *sql.Tx) (err error) {
		err = patchConfigsLegacy(tx, func(conf *config.Configs) {
			conf.CaptchaTags = config.Defaults.CaptchaTags
			conf.OverrideCaptchaTags = map[string]string{}
		})
		if err != nil {
			return
		}
		_, err = tx.Exec(`drop table captchas`)
		return
	},
	func(tx *sql.Tx) error {
		return execAll(tx,
			`create table last_solved_captchas (
				ip inet primary key,
				time timestamp not null default (now() at time zone 'utc')
			)`,
			createIndex("last_solved_captchas", "time"),
		)
	},
	func(tx *sql.Tx) error {
		// Moved
		return nil
	},
	func(tx *sql.Tx) error {
		// Moved
		return nil
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(`alter table images drop column apng`)
		return
	},
	func(tx *sql.Tx) (err error) {
		// Drop legacy functions and triggers
		err = dropFunctions(tx, "notify_thread_post_count",
			"notify_thread_deleted")
		if err != nil {
			return
		}

		err = registerFunctions(tx, "post_count", "post_op", "bump_thread",
			"use_image_token", "insert_image")
		if err != nil {
			return
		}

		_, err = tx.Exec("alter table mod_log rename column id to post_id")
		if err != nil {
			return
		}
		_, err = tx.Exec("alter table mod_log add column id serial primary key")
		if err != nil {
			return
		}

		return
	},
	func(tx *sql.Tx) (err error) {
		return execAll(tx,
			`alter table posts
				alter column id set default nextval('post_id'),
				alter column time set default extract(epoch from now()),
				drop column posterID`,
			`alter table threads
				alter column id set default nextval('post_id'),
				alter column replyTime set default extract(epoch from now()),
				alter column bumpTime set default extract(epoch from now())`,
			`alter table boards
				drop column posterIDs`,
		)
	},
	func(tx *sql.Tx) (err error) {
		return execAll(tx,
			`alter table images rename column fileType to file_type`,
			`alter table images rename column thumbType to thumb_type`,
		)
	},
	func(tx *sql.Tx) (err error) {
		modLevels := map[string]common.ModerationLevel{
			"janitors":   common.Janitor,
			"moderators": common.Moderator,
			"owners":     common.BoardOwner,
			"admin":      common.Admin,
		}

		modTable := func(table, column string) (err error) {
			return execAll(tx,
				fmt.Sprintf(`alter table %s drop column %s`, table, column),
				fmt.Sprintf(
					`alter table %s
						add column %s smallint not null default 0`,
					table, column),
			)
		}

		// Port all post moderation titles
		{
			titles := make(map[uint64]common.ModerationLevel)

			var r *sql.Rows
			r, err = sq.Select("id", "auth").
				From("posts").
				Where("auth is not null").
				RunWith(tx).
				Query()
			if err != nil {
				return
			}
			defer r.Close()

			var (
				id   uint64
				auth sql.NullString
			)
			for r.Next() {
				err = r.Scan(&id, &auth)
				if err != nil {
					return
				}
				pos := modLevels[auth.String]
				if pos >= common.Janitor {
					titles[id] = pos
				}
			}
			err = r.Err()
			if err != nil {
				return
			}

			err = modTable("posts", "auth")
			if err != nil {
				return
			}

			for id, pos := range titles {
				_, err = sq.Update("posts").
					Set("auth", int(pos)).
					Where("id = ?", id).
					RunWith(tx).
					Exec()
				if err != nil {
					return
				}
			}
		}

		// Port staff table
		{
			type row struct {
				account, board string
				position       common.ModerationLevel
			}

			var positions []row

			var r *sql.Rows
			r, err = sq.Delete("staff").
				Suffix("returning account, board, position").
				RunWith(tx).
				Query()
			if err != nil {
				return
			}
			defer r.Close()

			var account, board, pos string
			for r.Next() {
				err = r.Scan(&account, &board, &pos)
				if err != nil {
					return
				}
				positions = append(positions,
					row{account, board, modLevels[pos]})
			}
			err = r.Err()
			if err != nil {
				return
			}

			err = modTable("staff", "position")
			if err != nil {
				return
			}

			for _, r := range positions {
				_, err = sq.Insert("staff").
					Columns("account", "board", "position").
					Values(r.account, r.board, r.position).
					RunWith(tx).
					Exec()
				if err != nil {
					return
				}
			}
		}

		return
	},
	func(tx *sql.Tx) error {
		// Reload triggers
		return loadSQL(tx, "triggers/boards")
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(createIndex("post_moderation", "type"))
		if err != nil {
			return
		}
		return registerFunctions(tx, "delete_posts_by_ip", "assert_can_perform")
	},
	func(tx *sql.Tx) (err error) {
		return registerFunctions(tx, "is_deleted", "delete_posts_by_ip")
	},
	func(tx *sql.Tx) (err error) {
		err = execAll(tx,
			`create table continuous_deletions (
				ip inet not null,
				board text not null,
				by text not null,
				till timestamptz not null
			)`,
			createIndex("continuous_deletions", "ip", "board"),
			createIndex("continuous_deletions", "till"),
		)
		if err != nil {
			return
		}

		// Reload functions
		err = registerFunctions(tx, "post_board", "delete_posts_by_ip")
		if err != nil {
			return
		}

		// Enables the above by avoiding constraint violation
		_, err = tx.Exec(
			`alter table post_moderation
			alter constraint post_moderation_post_id_fkey
			deferrable initially immediate`)
		if err != nil {
			return
		}

		defaults := [...][2]string{
			{"length", "0"},
			{"data", "''"},
		}
		for _, pair := range defaults {
			err = setDefault(tx, "post_moderation", pair[0], pair[1])
			if err != nil {
				return
			}
		}

		return
	},
	func(tx *sql.Tx) (err error) {
		// Load new versions
		return registerFunctions(tx, "delete_posts_by_ip")
	},
	func(tx *sql.Tx) (err error) {
		// Moved
		return
	},
	func(tx *sql.Tx) (err error) {
		err = execAll(tx,
			`drop table continuous_deletions cascade`,
			`create type ban_type as enum ('classic', 'shadow')`,
			`alter table bans
				add column type ban_type not null default 'classic'`,
			createIndex("bans", "type"),
		)
		if err != nil {
			return
		}

		err = registerFunctions(tx, "delete_posts_by_ip")
		if err != nil {
			return
		}
		return loadSQL(tx, "triggers/mod_log", "triggers/posts")
	},
	func(tx *sql.Tx) (err error) {
		for _, args := range [...]string{
			`bigint, text`,
			`bigint, text, text`,
			`bigint, text, text, bigint`,
			`bigint, text, text, bigint, text`,
		} {
			err = dropFunctions(tx, fmt.Sprintf("delete_post(%s)", args))
			if err != nil {
				return
			}
		}
		return registerFunctions(tx, "delete_posts", "delete_images",
			"spoiler_images")
	},
	func(tx *sql.Tx) (err error) {
		return registerFunctions(tx, "delete_posts_by_ip")
	},
	func(tx *sql.Tx) (err error) {
		return registerFunctions(tx, "delete_images", "spoiler_images")
	},
	func(tx *sql.Tx) (err error) {
		for _, args := range [...]string{
			`bigint, boolean, boolean, boolean`,
			`bigint, character varying, boolean, boolean`,
			`bigint, character varying, bytea, boolean, boolean`,
			`bigint, boolean, boolean`,
			`bigint, boolean`,
		} {
			err = dropFunctions(tx, fmt.Sprintf("bump_thread(%s)", args))
			if err != nil {
				return
			}
		}
		return registerFunctions(tx, "bump_thread")
	},
	func(tx *sql.Tx) (err error) {
		// Make trigger naming conform to their execution times

		// Drop old trigger functions and their triggers
		for _, s := range [...]string{
			"boards_insert", "boards_update", "boards_delete",
			"mod_log_insert",
			"posts_insert", "posts_update",
			"threads_insert", "threads_update", "threads_delete",
		} {
			err = dropFunctions(tx, "on_"+s)
			if err != nil {
				return
			}
		}

		// Register the new triggers
		return registerTriggers(tx, map[string][]triggerDescriptor{
			"boards": {
				{after, tableInsert},
				{after, tableUpdate},
				{after, tableDelete},
			},
			"mod_log": {{after, tableInsert}},
			"posts":   {{before, tableInsert}, {after, tableUpdate}},
			"threads": {
				{after, tableInsert},
				{after, tableUpdate},
				{after, tableDelete},
			},
		})
	},
	func(tx *sql.Tx) (err error) {
		return execAll(
			tx,
			`alter table boards alter column id type varchar(10)`,
			`alter table boards alter column defaultcss type varchar(20)`,

			`alter table banners alter column board type varchar(10)`,

			`alter table bans alter column board type varchar(10)`,
			`alter table bans alter column reason type varchar(100)`,

			`alter table loading_animations alter column board type varchar(10)`,

			`alter table mod_log alter column board type varchar(10)`,
			`alter table mod_log alter column board type varchar(100)`,

			`alter table post_moderation alter column by type varchar(20)`,
			`alter table post_moderation alter column data type varchar(100)`,

			`alter table posts alter column board type varchar(10)`,
			`alter table posts alter column trip type varchar(100)`,
			`alter table posts alter column name type varchar(50)`,
			`alter table posts alter column imagename type varchar(200)`,
			`alter table posts alter column flag type varchar(2)`,

			`alter table pyu alter column id type varchar(10)`,

			`alter table pyu_limit alter column board type varchar(10)`,

			`alter table reports alter column board type varchar(10)`,
			`alter table reports alter column reason type varchar(100)`,

			`alter table staff alter column board type varchar(10)`,

			`alter table threads alter column board type varchar(10)`,
		)
	},
	func(tx *sql.Tx) (err error) {
		err = execAll(tx,
			`alter table threads rename column replytime to update_time`,
			`alter table threads rename column bumptime to bump_time`,
		)
		if err != nil {
			return
		}
		err = registerFunctions(tx, "bump_thread")
		if err != nil {
			return
		}
		return loadSQL(tx, "triggers/threads")
	},
	func(tx *sql.Tx) (err error) {
		_, err = sq.Delete("main").
			Where("id = 'geo_md5'").
			RunWith(tx).
			Exec()
		return
	},
	func(tx *sql.Tx) (err error) {
		return loadSQL(tx, "triggers/posts")
	},
	func(tx *sql.Tx) (err error) {
		return loadSQL(tx, "triggers/posts", "triggers/mod_log")
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(
			`alter table main
			alter column val
			type jsonb using (val::jsonb)`)
		return
	},
	func(tx *sql.Tx) (err error) {
		var salt string
		err = sq.Select("val->>'salt'").
			From("main").
			Where("id = 'config'").
			QueryRow().
			Scan(&salt)
		if err != nil {
			return
		}

		switch salt {
		case "", config.Defaults.Salt:
			salt, err = auth.RandomID(64)
			if err != nil {
				return
			}
			_, err = tx.Exec(
				`update main
				set val = val || jsonb_build_object('salt', $1::text)
				where id = 'config'`,
				salt,
			)
			return
		}
		return
	},
	func(tx *sql.Tx) (err error) {
		return execAll(tx,
			`drop table spam_scores`,
			`create table spam_scores(
				token bytea primary key,
				score bigint not null
			)`,
			`drop table last_solved_captchas`,
			`create table last_solved_captchas(
				token bytea primary key,
				time timestamptz not null default now()
			)`,
			// Solves potential cookie domain problems
			`update main
			set val = val || '{"rootURL":"http://127.0.0.1"}'
			where id = 'config'
				and val->>'rootURL' = 'http://localhost'`,
		)
	},
	func(tx *sql.Tx) (err error) {
		return execAll(tx,
			`create table bitchute_videos (
				id varchar(1000) primary key,
				title varchar(1000) not null
			)`,
		)
	},
	func(tx *sql.Tx) (err error) {
		return execAll(tx,
			`create table attempted_logins (
				ip inet not null,
				account varchar(20) not null,
				attempts smallint not null,
				expires timestamp not null,
				primary key (ip, account)
			)`,
			createIndex("attempted_logins", "expires"),
		)
	},
	func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(`drop table roulette`)
		if err != nil {
			return
		}
		return loadSQL(tx, "triggers/threads")
	},
	func(tx *sql.Tx) (err error) {
		return execAll(tx,
			`alter table posts alter column flag type varchar(5)`,
		)
	},
	func(tx *sql.Tx) error {
		return execAll(tx,
			`truncate table sessions;`,
		)
	},
	func(tx *sql.Tx) error {
		return execAll(tx,
			`alter table boards
			add randomNameHours bool not null default false`,
		)
	},
	func(tx *sql.Tx) error {
		// Reload trigger
		return registerTriggers(tx, map[string][]triggerDescriptor{
			"posts": {{before, tableInsert}},
		})
	},
}

func createIndex(table string, columns ...string) string {
	var w strings.Builder
	w.WriteString(table)
	for _, c := range columns {
		fmt.Fprintf(&w, "_%s", c)
	}
	w.WriteString("_idx")

	return fmt.Sprintf(`create index %s on %s (%s)`,
		w.String(), table, strings.Join(columns, ", "))
}

func setDefault(tx *sql.Tx, table, column, def string) (err error) {
	_, err = tx.Exec(
		fmt.Sprintf("alter table %s alter column %s set default %s",
			table, column, def),
	)
	return
}

func registerFunctions(tx *sql.Tx, files ...string) (err error) {
	for _, f := range files {
		err = loadSQL(tx, "functions/"+f)
		if err != nil {
			return
		}
	}
	return
}

func dropFunctions(tx *sql.Tx, names ...string) (err error) {
	for _, n := range names {
		_, err = tx.Exec(fmt.Sprintf("drop function if exists %s cascade", n))
		if err != nil {
			return
		}
	}
	return
}

func loadSQL(tx *sql.Tx, paths ...string) (err error) {
	var buf []byte
	for _, p := range paths {
		buf, err = static.ReadFile(fmt.Sprintf("/sql/%s.sql", p))
		if err != nil {
			return
		}
		_, err = tx.Exec(string(buf))
		if err != nil {
			err = util.WrapError(p, err)
			return
		}
	}
	return
}

// Run migrations, till the DB version matches the code version
func runMigrations() (err error) {
	for {
		var (
			currentVersion int
			done           bool
		)
		err = InTransaction(false, func(tx *sql.Tx) (err error) {
			// Lock version column to ensure no migrations from other processes
			// happen concurrently
			err = sq.Select("val").
				From("main").
				Where("id = 'version'").
				Suffix("for update").
				RunWith(tx).
				QueryRow().
				Scan(&currentVersion)
			if err != nil {
				return
			}
			if currentVersion == version {
				done = true
				return
			}
			if currentVersion > version {
				log.Fatal("database version ahead of codebase")
			}

			if !common.IsTest {
				log.Infof("upgrading database to version %d", currentVersion+1)
			}

			err = migrations[currentVersion](tx)
			if err != nil {
				return
			}

			// Write new version number
			_, err = sq.Update("main").
				Set("val", currentVersion+1).
				Where("id = 'version'").
				RunWith(tx).
				Exec()
			return
		})
		if err != nil {
			return fmt.Errorf("migration error: %d -> %d: %s",
				currentVersion, currentVersion+1, err)
		}
		if done {
			return
		}
	}
}

// Patches server configuration during upgrades.
//
// Legacy function. Only kept for migrations.
func patchConfigsLegacy(tx *sql.Tx, fn func(*config.Configs)) (err error) {
	var s string
	err = tx.QueryRow("SELECT val FROM main WHERE id = 'config'").Scan(&s)
	if err != nil {
		return
	}
	var conf config.Configs
	err = json.Unmarshal([]byte(s), &conf)
	if err != nil {
		return
	}

	fn(&conf)

	buf, err := json.Marshal(conf)
	if err != nil {
		return
	}
	_, err = tx.Exec(
		`UPDATE main
			SET val = $1
			WHERE id = 'config'`,
		string(buf),
	)
	return
}

// For decoding and encoding the tuple arrays we used to store links in.
// Still needed for migrations.
type linkRowLegacy [][2]uint64

func (l *linkRowLegacy) Scan(src interface{}) error {
	switch src := src.(type) {
	case []byte:
		return l.scanBytes(src)
	case string:
		return l.scanBytes([]byte(src))
	case nil:
		*l = nil
		return nil
	default:
		return fmt.Errorf("cannot convert %T to [][2]uint", src)
	}
}

func (l *linkRowLegacy) scanBytes(src []byte) error {
	length := len(src)
	if length < 6 {
		return errors.New("source too short")
	}

	src = src[1 : length-1]

	// Determine needed size and preallocate final array
	commas := 0
	for _, b := range src {
		if b == ',' {
			commas++
		}
	}
	*l = make(linkRowLegacy, 0, (commas-1)/2+1)

	var (
		inner bool
		next  [2]uint64
		err   error
		buf   = make([]byte, 0, 16)
	)
	for _, b := range src {
		switch b {
		case '{': // New tuple
			inner = true
			buf = buf[0:0]
		case ',':
			if inner { // End of first uint
				next[0], err = strconv.ParseUint(string(buf), 10, 64)
				if err != nil {
					return err
				}
				buf = buf[0:0]
			}
		case '}': // End of tuple
			next[1], err = strconv.ParseUint(string(buf), 10, 64)
			if err != nil {
				return err
			}
			*l = append(*l, next)
		default:
			buf = append(buf, b)
		}
	}

	return nil
}

func (l linkRowLegacy) Value() (driver.Value, error) {
	n := len(l)
	if n == 0 {
		return nil, nil
	}

	b := make([]byte, 1, 16)
	b[0] = '{'
	for i, l := range l {
		if i != 0 {
			b = append(b, ',')
		}
		b = append(b, '{')
		b = strconv.AppendUint(b, l[0], 10)
		b = append(b, ',')
		b = strconv.AppendUint(b, l[1], 10)
		b = append(b, '}')
	}
	b = append(b, '}')

	return string(b), nil
}
