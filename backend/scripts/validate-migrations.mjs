import { readdir } from "node:fs/promises";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";

const LEGACY_MAXIMUM = 234;
const MIGRATION_PATTERN = /^(\d{3})_[a-z0-9]+(?:_[a-z0-9]+)*\.sql$/;
const LEADING_IDENTIFIER_PATTERN = /^(\d+)/;

export async function validateMigrationRegistry(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const postBaseline = [];

  for (const entry of entries) {
    if (!entry.isFile() || !entry.name.endsWith(".sql")) continue;

    const leadingIdentifier = entry.name.match(LEADING_IDENTIFIER_PATTERN);
    if (!leadingIdentifier || Number(leadingIdentifier[1]) <= LEGACY_MAXIMUM) continue;

    const migration = entry.name.match(MIGRATION_PATTERN);
    if (!migration) {
      throw new Error(`invalid post-${LEGACY_MAXIMUM} migration name: ${entry.name}`);
    }
    postBaseline.push({ identifier: Number(migration[1]), name: entry.name });
  }

  postBaseline.sort((left, right) =>
    left.identifier - right.identifier || left.name.localeCompare(right.name),
  );

  let expected = LEGACY_MAXIMUM + 1;
  for (let index = 0; index < postBaseline.length; index += 1) {
    const migration = postBaseline[index];
    const previous = postBaseline[index - 1];
    if (previous?.identifier === migration.identifier) {
      throw new Error(`duplicate migration identifier ${migration.identifier}`);
    }
    if (migration.identifier !== expected) {
      throw new Error(`out-of-order migration identifier: expected ${expected}, found ${migration.identifier}`);
    }
    expected += 1;
  }

  return { legacyMaximum: LEGACY_MAXIMUM, validated: postBaseline.length };
}

async function main() {
  const directory = resolve(process.argv[2] ?? "migrations");
  const result = await validateMigrationRegistry(directory);
  process.stdout.write(
    `migration registry valid: legacy maximum ${result.legacyMaximum}, ${result.validated} post-baseline migration(s)\n`,
  );
}

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
  main().catch((error) => {
    process.stderr.write(`${error.message}\n`);
    process.exitCode = 1;
  });
}
