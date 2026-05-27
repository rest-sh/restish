import {mkdir, readFile, readdir, rm, writeFile} from "node:fs/promises";

import {Resvg} from "@resvg/resvg-js";
import YAML from "yaml";
import {fileURLToPath} from "node:url";
import path from "node:path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const siteDir = path.resolve(__dirname, "..");
const contentDir = path.join(siteDir, "content", "en");
const outputDir = path.join(siteDir, "static", "images", "social");
const fontDir = path.join(
  siteDir,
  "node_modules",
  "@fontsource",
  "space-grotesk",
  "files",
);
const spaceGroteskFonts = [400, 500, 600, 700].map((weight) =>
  path.join(fontDir, `space-grotesk-latin-${weight}-normal.woff`),
);

const width = 1200;
const height = 630;
const fallbackDescription =
  "API-aware HTTP from your terminal, with OpenAPI commands, auth, output shaping, and plugin support.";

const escapeXml = (value) =>
  String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");

const humanize = (value) =>
  value
    .replace(/^_index$/, "documentation")
    .replace(/[-_]+/g, " ")
    .replace(/\b\w/g, (char) => char.toUpperCase());

const textWidth = (value, fontSize) => {
  let units = 0;
  for (const char of value) {
    if (char === " ") units += 0.34;
    else if (/[il.,:;|!]/.test(char)) units += 0.28;
    else if (/[A-Z0-9{}()[\]/]/.test(char)) units += 0.68;
    else units += 0.56;
  }
  return units * fontSize;
};

const wrapText = (value, maxWidth, fontSize, maxLines) => {
  const words = String(value ?? "")
    .trim()
    .split(/\s+/)
    .filter(Boolean);
  const lines = [];
  let current = "";

  for (const word of words) {
    const candidate = current ? `${current} ${word}` : word;
    if (textWidth(candidate, fontSize) <= maxWidth) {
      current = candidate;
      continue;
    }
    if (current) lines.push(current);
    current = word;
    if (lines.length === maxLines) break;
  }

  if (current && lines.length < maxLines) lines.push(current);

  if (lines.length === maxLines) {
    const usedWords = lines.join(" ").split(/\s+/).length;
    if (usedWords < words.length) {
      let last = lines[lines.length - 1];
      while (last.length > 1 && textWidth(`${last}...`, fontSize) > maxWidth) {
        last = last.replace(/\s+\S*$/, "");
        if (!last.includes(" ")) last = last.slice(0, -1);
      }
      lines[lines.length - 1] = `${last.trim()}...`;
    }
  }

  return lines;
};

const parsePage = async (file) => {
  const source = await readFile(file, "utf8");
  if (!source.startsWith("---\n")) return {};

  const end = source.indexOf("\n---", 4);
  if (end === -1) return {};

  const frontMatter = source.slice(4, end);
  return YAML.parse(frontMatter) ?? {};
};

const listMarkdown = async (dir) => {
  const entries = await readdir(dir, {withFileTypes: true});
  const files = await Promise.all(
    entries.map(async (entry) => {
      const fullPath = path.join(dir, entry.name);
      if (entry.isDirectory()) return listMarkdown(fullPath);
      if (entry.isFile() && entry.name.endsWith(".md")) return [fullPath];
      return [];
    }),
  );
  return files.flat().sort();
};

const socialPathFor = (contentPath) => {
  const relative = path
    .relative(contentDir, contentPath)
    .replaceAll(path.sep, "/");
  let stem = relative.replace(/\.md$/, "").replace(/\/_index$/, "");
  if (stem === "_index" || stem === "") stem = "index";
  return `${stem}.png`;
};

const sectionFor = (socialPath) => {
  const segments = socialPath.replace(/\.png$/, "").split("/");
  if (segments[0] === "docs" && segments[1]) return humanize(segments[1]);
  if (segments[0] === "docs") return "Documentation";
  return "Restish";
};

const renderSvg = ({
  title,
  description,
  section,
  command = "$ restish docs",
  descMaxLines,
}) => {
  const titleFontSize = title.length > 32 ? 58 : 68;
  const titleLineHeight = titleFontSize + 10;
  const titleLines = wrapText(title, 610, titleFontSize, 3);
  const resolvedDescMaxLines = descMaxLines ?? Math.max(3, 6 - titleLines.length);
  const descLines = wrapText(description, 610, 30, resolvedDescMaxLines);
  const titleY = 214;
  const descY = titleY + titleLines.length * titleLineHeight + 30;

  return `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}" viewBox="0 0 ${width} ${height}">
  <defs>
    <linearGradient id="bg" x1="0" y1="0" x2="1200" y2="630" gradientUnits="userSpaceOnUse">
      <stop offset="0" stop-color="#020617"/>
      <stop offset="0.58" stop-color="#111827"/>
      <stop offset="1" stop-color="#0f2a33"/>
    </linearGradient>
    <filter id="terminalGlow" x="-14%" y="-10%" width="128%" height="142%" color-interpolation-filters="sRGB">
      <feDropShadow dx="2" dy="17" stdDeviation="10" flood-color="#ff4dbd" flood-opacity="0.17"/>
      <feDropShadow dx="0" dy="8" stdDeviation="7" flood-color="#ff4dbd" flood-opacity="0.08"/>
    </filter>
  </defs>
  <rect width="1200" height="630" fill="url(#bg)"/>
  <path d="M0 510 C180 462 276 548 438 494 C622 432 698 510 874 450 C1028 398 1118 422 1200 366 L1200 630 L0 630 Z" fill="#2dd4bf" opacity="0.10"/>
  <path d="M0 94 H1200 M0 188 H1200 M0 282 H1200 M0 376 H1200 M0 470 H1200 M172 0 V630 M344 0 V630 M516 0 V630 M688 0 V630 M860 0 V630 M1032 0 V630" stroke="#94a3b8" stroke-opacity="0.08" stroke-width="1"/>

  <g transform="translate(78 74)">
    <text x="0" y="0" dominant-baseline="hanging" fill="#5eead4" font-family="Space Grotesk, Arial, Helvetica, sans-serif" font-size="24" font-weight="700" letter-spacing="3">${escapeXml(section.toUpperCase())}</text>
    <text x="0" y="44" dominant-baseline="hanging" fill="#fbbf24" font-family="ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace" font-size="22" font-weight="700">${escapeXml(command)}</text>
  </g>

  <g transform="translate(78 0)">
    ${titleLines
      .map(
        (line, index) =>
          `<text x="0" y="${titleY + index * titleLineHeight}" fill="#f8fafc" font-family="Space Grotesk, Arial, Helvetica, sans-serif" font-size="${titleFontSize}" font-weight="700">${escapeXml(line)}</text>`,
      )
      .join("\n    ")}
    ${descLines
      .map(
        (line, index) =>
          `<text x="0" y="${descY + index * 40}" fill="#cbd5e1" font-family="Space Grotesk, Arial, Helvetica, sans-serif" font-size="30" font-weight="400">${escapeXml(line)}</text>`,
      )
      .join("\n    ")}
  </g>

  <g transform="translate(770 150)">
    <rect x="0" y="0" width="350" height="330" rx="18" fill="#020617" filter="url(#terminalGlow)" opacity="0.98"/>
    <rect x="0" y="0" width="350" height="330" rx="18" fill="#020617" stroke="#334155" stroke-width="2"/>
    <rect x="0" y="0" width="350" height="58" rx="18" fill="#0f172a"/>
    <path d="M0 40 Q0 58 18 58 H332 Q350 58 350 40 V58 H0 Z" fill="#0f172a"/>
    <circle cx="32" cy="29" r="8" fill="#ff4dbd"/>
    <circle cx="58" cy="29" r="8" fill="#fbbf24"/>
    <circle cx="84" cy="29" r="8" fill="#2dd4bf"/>
    <text x="213" y="38" text-anchor="middle" font-family="Space Grotesk, Arial, Helvetica, sans-serif" font-size="26" font-weight="700">
      <tspan fill="#5eead4" font-size="33">{</tspan><tspan dx="0.22em" fill="#f8fafc">rest</tspan><tspan dx="0.08em" fill="#ff4dbd">!</tspan><tspan dx="0.04em" fill="#f8fafc">sh</tspan><tspan dx="0.12em" fill="#5eead4" font-size="33">}</tspan>
    </text>
    <g transform="translate(30 92)">
      <rect x="0" y="0" width="80" height="14" rx="7" fill="#5eead4"/>
      <rect x="98" y="0" width="178" height="14" rx="7" fill="#475569"/>
      <rect x="0" y="40" width="240" height="14" rx="7" fill="#e2e8f0"/>
      <rect x="258" y="40" width="38" height="14" rx="7" fill="#ff4dbd"/>
      <rect x="0" y="80" width="132" height="14" rx="7" fill="#fbbf24"/>
      <rect x="150" y="80" width="118" height="14" rx="7" fill="#475569"/>
      <rect x="0" y="120" width="274" height="14" rx="7" fill="#64748b"/>
      <rect x="0" y="160" width="92" height="14" rx="7" fill="#ff4dbd"/>
      <rect x="110" y="160" width="156" height="14" rx="7" fill="#e2e8f0"/>
      <rect x="0" y="200" width="220" height="14" rx="7" fill="#2dd4bf"/>
    </g>
  </g>
</svg>`;
};

const renderPng = async (socialPath, svg) => {
  const png = new Resvg(svg, {
    fitTo: {mode: "width", value: width},
    font: {
      fontFiles: spaceGroteskFonts,
      loadSystemFonts: true,
    },
  })
    .render()
    .asPng();

  const destination = path.join(outputDir, socialPath);
  await mkdir(path.dirname(destination), {recursive: true});
  await writeFile(destination, png);
  return socialPath;
};

const renderPage = async (file) => {
  const frontMatter = await parsePage(file);
  if (frontMatter.draft === true) return null;

  const socialPath = socialPathFor(file);
  const title =
    frontMatter.title ?? humanize(path.basename(socialPath, ".png"));
  const description = frontMatter.description ?? fallbackDescription;
  const section = sectionFor(socialPath);
  return renderPng(socialPath, renderSvg({title, description, section}));
};

const renderGithubOverview = () =>
  renderPng(
    "github.png",
    renderSvg({
      title: "Restish CLI",
      description:
        "Explore REST-ish APIs with generic HTTP verbs, generated OpenAPI commands, shorthand input, auth, response filtering & projection, pagination, and plugins.",
      section: "Always Free. Always Open Source.",
      command: "$ brew install rest-sh/tap/restish",
      descMaxLines: 5,
    }),
  );

const main = async () => {
  const files = await listMarkdown(contentDir);
  await rm(outputDir, {recursive: true, force: true});

  const rendered = [];
  for (const file of files) {
    const socialPath = await renderPage(file);
    if (socialPath) rendered.push(socialPath);
  }
  rendered.push(await renderGithubOverview());

  console.log(
    `Generated ${rendered.length} social preview images in ${path.relative(siteDir, outputDir)}`,
  );
};

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
