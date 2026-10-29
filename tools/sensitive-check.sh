#!/usr/bin/env bash
# sensitive-check.sh — 공개 저장소에 비밀값·사내 정보가 섞였는지 검사한다.
#
#   sensitive-check.sh --staged            git diff --cached 의 추가 줄 (pre-commit 용)
#   sensitive-check.sh --tree <dir>        작업 트리 전체 파일
#   sensitive-check.sh --history <repo>    git log -p --all 전체 (추가 줄 + 커밋 메시지 + author/committer)
#
#   옵션: --allow <file>   .sensitive-allow 대신 쓸 예외 파일
#         --show-line      걸린 줄을 값만 마스킹해서 함께 출력 (분류용)
#         --raw            값을 마스킹하지 않고 출력. .sensitive-allow 를 기계로 만들 때만 쓰고, 파일로 돌리며 화면·보고서에 붙이지 않는다
#
# 패턴: writing-method 05-file-placement.md §11.1 + deny 파일(회사명·고객사·사내 도메인 등)
#   deny 파일: $SENSITIVE_DENY_FILE 또는 ~/.config/runners-lab/deny.txt
#              한 줄에 정규식 하나, 대소문자 무시, '#' 주석. 없으면 경고하고 §11.1 패턴만 쓴다.
# 예외: 저장소 루트 .sensitive-allow — 한 줄에 정규식 하나('#' 주석). 걸린 값(match)에 맞으면 건너뛴다.
#
# 출력: <위치>:<줄> <패턴 이름> <값 앞 4자 + ****>   (위치: 파일 경로, history 는 <커밋7자>:<경로>)
# 종료 코드: 0 = 0건, 1 = 걸린 것 있음, 2 = 사용법 오류
#
# 예외 파일(.sensitive-allow) 자체는 검사하지 않는다 — 걸린 값을 정규식으로 적어 두므로 자기 자신에 다시 걸린다.
# 의존: bash, git, perl(macOS 기본 /usr/bin/perl). 줄은 앞 20000자만 본다.
set -euo pipefail
exec perl -x "$0" "$@"
#!perl
use strict;
use warnings;
use File::Find;
use Cwd qw(abs_path);

my ($mode, $target, $allow_file, $show_line, $raw) = ('', '', '', 0, 0);
while (@ARGV) {
    my $a = shift @ARGV;
    if    ($a eq '--staged')    { $mode = 'staged' }
    elsif ($a eq '--tree')      { $mode = 'tree';    $target = shift(@ARGV) // '' }
    elsif ($a eq '--history')   { $mode = 'history'; $target = shift(@ARGV) // '' }
    elsif ($a eq '--allow')     { $allow_file = shift(@ARGV) // '' }
    elsif ($a eq '--show-line') { $show_line = 1 }
    elsif ($a eq '--raw')       { $raw = 1 }
    elsif ($a eq '-h' || $a eq '--help') { usage(0) }
    else { print STDERR "알 수 없는 인자: $a\n"; usage(2) }
}
usage(2) unless $mode;
usage(2) if $mode ne 'staged' && $target eq '';

sub usage {
    print STDERR "사용법: sensitive-check.sh --staged | --tree <dir> | --history <repo> [--allow <file>] [--show-line] [--raw]\n";
    exit $_[0];
}

# ---- 패턴 -------------------------------------------------------------
my $PLACEHOLDER = qr/(?![\$\{<%\(\[\s"'])/;    # $VAR, {{x}}, <token>, %s 같은 자리표시자는 값으로 보지 않는다
my @PAT = (
    ['aws-access-key',   qr/AKIA[0-9A-Z]{16}/],
    ['google-api-key',   qr/AIza[0-9A-Za-z_-]{35}/],
    ['slack-token',      qr/xox[baprs]-[0-9a-zA-Z-]{10,48}/],
    ['jwt',              qr/eyJ[A-Za-z0-9_-]+\.eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+/],
    ['bearer-header',    qr/Authorization:\s*Bearer\s+$PLACEHOLDER\S+/i],
    ['github-token',     qr/(?:ghp|gho|ghs|ghu|github_pat)_[A-Za-z0-9_]{16,}/],
    ['sk-api-key',       qr/\bsk-[A-Za-z0-9_-]{20,}/],
    ['aws-key-value',    qr/(?:aws_access_key_id|aws_secret_access_key)\s*=\s*\S+/i],
    ['pem-private-key',  qr/-----BEGIN [A-Z ]*PRIVATE KEY-----/],
    ['url-credential',   qr{[a-z][a-z0-9+.-]*://[^/\s:@]+:$PLACEHOLDER[^/\s:@]{3,}@}i],
    ['credential-assign',qr/(?:password|passwd|pwd|secret|api[_-]?key|access[_-]?key|token|credential)s?["']?\s*[=:]\s*["']?$PLACEHOLDER[A-Za-z0-9_!\@#\$%^&*+\/=.~-]{3,}(?![A-Za-z0-9_.]*\()/i],
    ['email',            qr/[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}/],
    ['kr-rrn',           qr/\b\d{6}-\d{7}\b/],
    ['private-ip-10',    qr/(?<![\d.])10(?:\.\d{1,3}){3}(?!\d)/],
    ['private-ip-192',   qr/(?<![\d.])192\.168(?:\.\d{1,3}){2}(?!\d)/],
    ['private-ip-172',   qr/(?<![\d.])172\.(?:1[6-9]|2\d|3[01])(?:\.\d{1,3}){2}(?!\d)/],
);
# 경로에만 적용하는 패턴 (파일 이름 자체가 비밀 후보)
my @PATH_PAT = (
    ['secret-filename', qr{(?:^|/)\.env(?:\.(?!example$|sample$|template$)[^/]*)?$}],
    ['secret-filename', qr{\.(?:pem|p12|pfx|jks|keystore)$}i],
    ['secret-filename', qr{(?:^|/)(?:id_(?:rsa|ed25519|ecdsa|dsa)|\.netrc)$}],
);

# ---- deny / allow -----------------------------------------------------
my $deny_path = $ENV{SENSITIVE_DENY_FILE} // (($ENV{HOME} // '') . '/.config/runners-lab/deny.txt');
my @DENY;    # [번호, qr]
if (open(my $df, '<:encoding(UTF-8)', $deny_path)) {
    my $n = 0;
    while (my $l = <$df>) {
        $n++;
        chomp $l; $l =~ s/\r$//;
        next if $l =~ /^\s*(?:#|$)/;
        my $re = eval { qr/$l/i };
        if ($re) { push @DENY, ["deny#$n", $re] } else { print STDERR "deny 파일 $n 줄 정규식 오류, 건너뜀\n" }
    }
    close $df;
} else {
    print STDERR "경고: deny 파일이 없다($deny_path). 회사명·사내 도메인 검사는 건너뛰고 §11.1 패턴만 쓴다.\n";
}

my @ALLOW;
sub load_allow {
    my ($file) = @_;
    return unless -f $file;
    open(my $af, '<:encoding(UTF-8)', $file) or return;
    while (my $l = <$af>) {
        chomp $l; $l =~ s/\r$//;
        next if $l =~ /^\s*(?:#|$)/;
        my $re = eval { qr/$l/ };
        push @ALLOW, $re if $re;
    }
    close $af;
}

# ---- 검사 엔진 --------------------------------------------------------
my $found = 0;
my %seen;

# 입력은 디코드된 문자열, 출력은 UTF-8 바이트 (경로·메시지는 처음부터 바이트라서 STDOUT 에 인코딩 층을 두지 않는다)
sub mask {
    my ($v) = @_;
    my $m = $raw ? $v : length($v) <= 4 ? '****' : substr($v, 0, 4) . '****';
    utf8::encode($m);
    return $m;
}

sub allowed {
    my ($v) = @_;
    for my $re (@ALLOW) { return 1 if $v =~ $re }
    return 0;
}

# 한 줄을 모든 패턴으로 검사한다. $dedupe 가 있으면 같은 (패턴, 값, $dedupe) 는 한 번만 보고한다.
sub scan_text {
    my ($loc, $ln, $text, $dedupe, $pats) = @_;
    $text = substr($text, 0, 20000);
    utf8::decode($text);
    my @hits;
    for my $p (@$pats) {
        my ($name, $re) = @$p;
        while ($text =~ /$re/g) {
            my ($s, $e) = ($-[0], $+[0]);
            my $val = substr($text, $s, $e - $s);
            next if allowed($val);
            if (defined $dedupe) { next if $seen{"$name\0$val\0$dedupe"}++ }
            print "$loc:$ln $name " . mask($val) . "\n";
            push @hits, [$s, $e, $val];
            $found++;
        }
    }
    if ($show_line && @hits) {
        my $masked = $text;
        for my $h (sort { $b->[0] <=> $a->[0] } @hits) {
            substr($masked, $h->[0], $h->[1] - $h->[0], mask($h->[2]));
        }
        $masked =~ s/^\s+//;
        $masked = substr($masked, 0, 200);
        utf8::encode($masked);
        print "    | $masked\n";
    }
}

sub scan_line { my ($loc, $ln, $text, $dedupe) = @_; scan_text($loc, $ln, $text, $dedupe, [@PAT, @DENY]) }
sub scan_path { my ($loc, $path, $dedupe) = @_; scan_text($loc, 0, $path, $dedupe, [@PATH_PAT, @DENY]) }

# ---- diff 스트림 (staged / history 공용) ------------------------------
sub scan_diff_stream {
    my ($fh, $is_history) = @_;
    my ($commit, $phase, $file, $ln, $inhunk, $mln, $idn) = ('', 'diff', '', 0, 0, 0, 0);
    while (defined(my $line = <$fh>)) {
        chomp $line;
        if ($is_history && $line =~ /^\@\@\@COMMIT (\w+)$/) {
            $commit = substr($1, 0, 7) . ':'; $phase = 'ident'; $idn = 0; $inhunk = 0; next;
        }
        if ($phase eq 'ident') {
            if ($line =~ /^\@\@\@IDENT (.*)$/) {
                my $label = $idn++ == 0 ? '(author)' : '(committer)';
                scan_line("$commit$label", 0, $1, 'ident');
                next;
            }
            if ($line eq '@@@MSGSTART') { $phase = 'msg'; $mln = 0; next }
        }
        if ($phase eq 'msg') {
            if ($line =~ /^diff --git /) { $phase = 'diff' }
            else { $mln++; scan_line("$commit(msg)", $mln, $line); next }
        }
        # diff 본문
        if ($line =~ /^diff --git /) { $inhunk = 0; $file = ''; next }
        if (!$inhunk) {
            if ($line =~ m{^\+\+\+ b/(.*)$}) { $file = $1; scan_path("$commit$file", $file, "path:$file"); next }
            if ($line =~ m{^\+\+\+ }) { $file = ''; next }
            if ($line =~ /^\@\@ -\d+(?:,\d+)? \+(\d+)/) { $ln = $1; $inhunk = 1 }
            next;
        }
        if ($line =~ /^\@\@ -\d+(?:,\d+)? \+(\d+)/) { $ln = $1; next }
        if ($line =~ /^\+(.*)$/) { scan_line("$commit$file", $ln, $1) unless $file eq '.sensitive-allow'; $ln++ }
    }
}

# ---- 모드별 실행 ------------------------------------------------------
sub git_toplevel {
    my ($dir) = @_;
    my $top = `git -C '$dir' rev-parse --show-toplevel 2>/dev/null`;
    chomp $top;
    return $top;
}

if ($mode eq 'staged') {
    my $top = git_toplevel('.') or do { print STDERR "git 저장소가 아니다\n"; exit 2 };
    load_allow($allow_file || "$top/.sensitive-allow");
    open(my $fh, '-|', 'git', '-C', $top, 'diff', '--cached', '-U0', '--no-color', '--no-ext-diff', '--diff-filter=ACMR') or die "git 실행 실패: $!";
    scan_diff_stream($fh, 0);
    close $fh;
}
elsif ($mode eq 'history') {
    -d $target or do { print STDERR "저장소 경로가 아니다: $target\n"; exit 2 };
    my $top = git_toplevel($target) or do { print STDERR "git 저장소가 아니다: $target\n"; exit 2 };
    load_allow($allow_file || "$top/.sensitive-allow");
    open(my $fh, '-|', 'git', '-C', $top, 'log', '-p', '--all', '-U0', '--no-color', '--no-ext-diff',
         '--format=@@@COMMIT %H%n@@@IDENT %an <%ae>%n@@@IDENT %cn <%ce>%n@@@MSGSTART%n%B') or die "git 실행 실패: $!";
    scan_diff_stream($fh, 1);
    close $fh;
}
else {
    -d $target or do { print STDERR "디렉터리가 아니다: $target\n"; exit 2 };
    my $root = abs_path($target);
    load_allow($allow_file || "$root/.sensitive-allow");
    find({
        no_chdir => 1,
        preprocess => sub { grep { $_ ne '.git' } @_ },
        wanted => sub {
            my $p = $File::Find::name;
            return if -l $p || !-f $p;
            (my $rel = $p) =~ s{^\Q$root\E/?}{};
            scan_path($rel, $rel, undef);
            return if $rel eq '.sensitive-allow';
            my $size = -s $p;
            if ($size > 5 * 1024 * 1024) { print STDERR "건너뜀(5MB 초과): $rel\n"; return }
            open(my $fh, '<:raw', $p) or return;
            read($fh, my $head, 8000);
            if (defined $head && index($head, "\0") >= 0) { close $fh; return }
            seek($fh, 0, 0);
            my $n = 0;
            while (defined(my $l = <$fh>)) { $n++; chomp $l; scan_line($rel, $n, $l) }
            close $fh;
        },
    }, $root);
}

print STDERR "sensitive-check: $found 건\n";
exit($found ? 1 : 0);
