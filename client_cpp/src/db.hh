#pragma once

#include <string>
#include <vector>

// Open a connection to the IndexedDB database and load data for specific
// threads
void load_db(std::vector<uint64_t> thread_ids);

// Handle a database error
void handle_db_error(std::string err);

// Load post ID sets from the database for the passed threads
void load_post_ids(const std::vector<uint64_t>& threads);

// Signals the database is ready. Called from the JS side.
void db_is_ready();
