#!/usr/bin/env node
'use strict';

// Installer for the capigo-api agent skill.
//
// Copies the bundled skill into a target directory that a skill-aware agent
// runtime loads from (e.g. ~/.openclaw/plugin-skills). The install is
// idempotent: the existing copy is removed first so a reference file dropped in
// a newer version can't linger as an orphan.
//
// Usage:
//   npx @vtech-com/capigo-skill --dir <skills-directory>
//   CAPIGO_SKILLS_DIR=<path> npx @vtech-com/capigo-skill

const fs = require('fs');
const os = require('os');
const path = require('path');

const SKILL_NAME = 'capigo-api';
const args = process.argv.slice(2);

function getFlag(name) {
  const i = args.indexOf(name);
  return i >= 0 ? args[i + 1] : undefined;
}

// Expand a leading "~" if the path wasn't already expanded by the caller's shell
// (e.g. when passed quoted or via an env var).
function expandHome(p) {
  if (p === '~') return os.homedir();
  if (p.startsWith('~/')) return path.join(os.homedir(), p.slice(2));
  return p;
}

const wantsHelp = args.includes('--help') || args.includes('-h');
const dirArg = getFlag('--dir') || process.env.CAPIGO_SKILLS_DIR;

if (wantsHelp || !dirArg) {
  const msg = `Install the capigo-api agent skill.

Usage:
  npx @vtech-com/capigo-skill --dir <skills-directory>
  CAPIGO_SKILLS_DIR=<path> npx @vtech-com/capigo-skill

Options:
  --dir <path>   Directory your agent runtime loads skills from,
                 e.g. ~/.openclaw/plugin-skills. May also be set via
                 the CAPIGO_SKILLS_DIR environment variable.
  -h, --help     Show this help.

The skill is installed at <skills-directory>/${SKILL_NAME} (idempotent —
any existing copy is replaced).`;
  console.log(msg);
  // Missing --dir is a usage error; an explicit --help is success.
  process.exit(wantsHelp ? 0 : 1);
}

const src = path.join(__dirname, '..', 'skill', SKILL_NAME);
if (!fs.existsSync(src)) {
  console.error(
    `error: bundled skill not found at ${src}. This package looks corrupt; ` +
      `reinstall with \`npx @vtech-com/capigo-skill@latest\`.`,
  );
  process.exit(1);
}

const destRoot = path.resolve(expandHome(dirArg));
const dest = path.join(destRoot, SKILL_NAME);

try {
  fs.rmSync(dest, { recursive: true, force: true });
  fs.mkdirSync(destRoot, { recursive: true });
  fs.cpSync(src, dest, { recursive: true });
} catch (err) {
  console.error(`error: failed to install skill to ${dest}: ${err.message}`);
  process.exit(1);
}

console.log(`Installed ${SKILL_NAME} skill → ${dest}`);
