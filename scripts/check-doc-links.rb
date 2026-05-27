#!/usr/bin/env ruby
# frozen_string_literal: true

require "find"
require "pathname"
require "set"

root = Pathname(ARGV[0] || "site/content/en/docs")
unless root.directory?
  warn "docs link check: #{root} is not a directory"
  exit 2
end

files = []
Find.find(root.to_s) do |path|
  files << Pathname(path) if path.end_with?(".md")
end

def page_url(root, file)
  rel = file.relative_path_from(root.parent).to_s
  url = "/" + rel.sub(%r{_index\.md$}, "").sub(/\.md$/, "/")
  url.gsub(%r{/+}, "/")
end

def front_matter(text)
  text[/\A---\n(.*?)\n---/m, 1] || ""
end

def aliases_from(text)
  aliases = []
  in_aliases = false
  front_matter(text).each_line do |line|
    if line.match?(/^aliases:\s*$/)
      in_aliases = true
      next
    end

    if in_aliases
      if (match = line.match(/^\s+-\s+(.+?)\s*$/))
        aliases << normalize_doc_path(match[1].strip.delete_prefix('"').delete_suffix('"').delete_prefix("'").delete_suffix("'"))
      elsif !line.start_with?(" ") && !line.strip.empty?
        in_aliases = false
      end
    end
  end
  aliases
end

def normalize_doc_path(path)
  clean = path.split("#", 2).first.to_s
  return "" if clean.empty?

  clean += "/" unless clean.end_with?("/") || File.extname(clean) != ""
  clean
end

def local_link?(href)
  return false if href.empty?
  return false if href.start_with?("#")
  return false if href.match?(%r{\A[a-z][a-z0-9+.-]*:}i)

  href.start_with?("/") || href.start_with?(".")
end

pages = {}
aliases = {}
file_urls = {}

files.each do |file|
  text = file.read
  url = page_url(root, file)
  pages[url] = file
  file_urls[file] = url
  aliases_from(text).each do |alias_url|
    aliases[alias_url] = file unless alias_url.empty?
  end
end

missing = []

files.each do |file|
  base = Pathname(file_urls[file])
  file.read.scan(/\[[^\]]+\]\(([^)]+)\)/).flatten.each do |raw_href|
    href = raw_href.split("#", 2).first.to_s
    next unless local_link?(href)

    path = if href.start_with?("/")
      normalize_doc_path(href)
    else
      normalize_doc_path(base.join(href).cleanpath.to_s)
    end

    next if pages.key?(path) || aliases.key?(path)

    missing << [file, raw_href, path]
  end
end

if missing.empty?
  puts "docs link check: ok (#{files.length} files, #{pages.length} pages, #{aliases.length} aliases)"
  exit 0
end

warn "docs link check: #{missing.length} missing local link(s)"
missing.each do |file, href, path|
  warn "#{file}: #{href} -> #{path}"
end
exit 1
