#!/usr/bin/env ruby
# frozen_string_literal: true

require "fileutils"
require "find"
require "open3"
require "optparse"
require "pathname"
require "shellwords"
require "tmpdir"
require "timeout"

options = {
  mode: "static",
  root: Pathname("site/content/en/docs"),
  restish: nil
}

OptionParser.new do |parser|
  parser.banner = "usage: scripts/check-doc-examples.rb [--mode static|live] [--root PATH] [--restish PATH]"
  parser.on("--mode MODE", "static or live") { |value| options[:mode] = value }
  parser.on("--root PATH", "docs root to scan") { |value| options[:root] = Pathname(value) }
  parser.on("--restish PATH", "restish binary for live mode") { |value| options[:restish] = value }
end.parse!

unless %w[static live].include?(options[:mode])
  warn "docs example check: --mode must be static or live"
  exit 2
end

root = options[:root]
unless root.directory?
  warn "docs example check: #{root} is not a directory"
  exit 2
end

Example = Struct.new(:file, :line, :raw, :command, keyword_init: true)

def markdown_files(root)
  files = []
  Find.find(root.to_s) do |path|
    files << Pathname(path) if path.end_with?(".md")
  end
  files.sort
end

def normalize_shortcode_command(raw)
  raw.strip
     .gsub(/\s*\\\s*\r?\n\s*/, " ")
     .gsub(/\r?\n\s*/, " ")
     .strip
end

def restish_examples(root)
  examples = []
  markdown_files(root).each do |file|
    text = file.read
    text.to_enum(:scan, /\{\{<\s*restish-example\s*>\}\}(.*?)\{\{<\s*\/restish-example\s*>\}\}/m).each do
      match = Regexp.last_match
      raw = match[1]
      line = text[0...match.begin(0)].count("\n") + 1
      examples << Example.new(file: file, line: line, raw: raw, command: normalize_shortcode_command(raw))
    end
  end
  examples
end

def parse_command(example)
  Shellwords.split(example.command)
rescue ArgumentError => e
  raise "#{example.file}:#{example.line}: shell parse failed: #{e.message}"
end

def static_check(examples)
  failures = []

  examples.each do |example|
    raw_lines = example.raw.lines.map(&:strip).reject(&:empty?)
    failures << "#{example.file}:#{example.line}: restish-example must contain exactly one command line" unless raw_lines.length == 1
    failures << "#{example.file}:#{example.line}: command must start with `restish`" unless example.command.start_with?("restish ")
    failures << "#{example.file}:#{example.line}: command contains command substitution" if example.command.include?("$(") || example.command.include?("`")

    begin
      tokens = parse_command(example)
      failures << "#{example.file}:#{example.line}: command must invoke `restish`" unless tokens.first == "restish"
      shell_operators = %w[| ; && || > >> < 2> 2>&1]
      found = tokens.find { |token| shell_operators.include?(token) }
      failures << "#{example.file}:#{example.line}: restish-example must be one direct invocation, found shell operator `#{found}`" if found
    rescue RuntimeError => e
      failures << e.message
    end
  end

  if failures.empty?
    puts "docs example check: ok (#{examples.length} restish-example commands)"
    true
  else
    warn "docs example check: #{failures.length} failure(s)"
    failures.each { |failure| warn failure }
    false
  end
end

def run_checked(env, argv, timeout_seconds: 30, expect_success: true)
  stdout = +""
  stderr = +""
  status = nil
  Timeout.timeout(timeout_seconds) do
    stdout, stderr, status = Open3.capture3(env, *argv)
  end

  ok = expect_success ? status.success? : !status.success?
  return nil if ok

  expectation = expect_success ? "succeeded" : "failed"
  "expected #{argv.shelljoin} to have #{expectation}, got exit #{status.exitstatus}\nstdout:\n#{stdout.byteslice(0, 1200)}\nstderr:\n#{stderr.byteslice(0, 1200)}"
rescue Timeout::Error
  "timed out after #{timeout_seconds}s: #{argv.shelljoin}"
end

def build_binary(root, package, output)
  _stdout, stderr, status = Open3.capture3("go", "build", "-o", output, package, chdir: root.to_s)
  return if status.success?

  raise "build #{package} failed:\n#{stderr}"
end

def live_skip_reason(command)
  return "interactive edit workflow" if command.start_with?("restish edit ")

  nil
end

def live_command(command, run_id)
  command.gsub(/key=(tour|docs-retry|docs-once|docs)(?=['&\s]|$)/, "key=docs-live-#{run_id}-\\1")
end

def expected_success?(command)
  return false if command.include?("api.rest.sh/slow?delay=2s") && command.include?("--rsh-timeout 500ms")
  return false if command.include?("api.rest.sh/flaky?failures=1&key=docs-once") && command.include?("--rsh-retry 0")

  true
end

def live_check(examples, repo_root, restish_path)
  failures = []
  skipped = 0

  Dir.mktmpdir("restish-doc-examples-") do |dir|
    temp = Pathname(dir)
    restish = restish_path || temp.join("restish").to_s
    build_binary(repo_root, "./cmd/restish", restish) unless restish_path

    plugin_dir = temp.join("plugins")
    FileUtils.mkdir_p(plugin_dir)
    build_binary(repo_root, "./cmd/restish-csv", plugin_dir.join("restish-csv").to_s)

    env = ENV.to_h.reject { |key, _| key.start_with?("RSH_") }
    config_path = temp.join("restish.json")
    config_path.write("{}\n")
    config_path.chmod(0o600)

    env["RSH_CONFIG"] = config_path.to_s
    env["RSH_CACHE_DIR"] = temp.join("cache").to_s
    env["NO_COLOR"] = "1"
    env["TERM"] = "dumb"

    setup_error = run_checked(env, [restish, "api", "connect", "example", "api.rest.sh", "--yes"], timeout_seconds: 45)
    raise setup_error if setup_error

    run_id = "#{Time.now.to_i}-#{Process.pid}"
    examples.each do |example|
      if (reason = live_skip_reason(example.command))
        skipped += 1
        warn "skip #{example.file}:#{example.line}: #{reason}"
        next
      end

      command = live_command(example.command, run_id)
      tokens = Shellwords.split(command)
      argv = [restish] + tokens.drop(1)
      error = run_checked(env, argv, timeout_seconds: 45, expect_success: expected_success?(example.command))
      failures << "#{example.file}:#{example.line}: #{error}" if error
    end
  end

  if failures.empty?
    puts "docs example live check: ok (#{examples.length - skipped} run, #{skipped} skipped)"
    true
  else
    warn "docs example live check: #{failures.length} failure(s)"
    failures.each { |failure| warn failure }
    false
  end
rescue RuntimeError => e
  warn "docs example live check: #{e.message}"
  false
end

examples = restish_examples(root)

unless static_check(examples)
  exit 1
end

if options[:mode] == "live"
  repo_root = Pathname.pwd
  exit 1 unless live_check(examples, repo_root, options[:restish])
end
