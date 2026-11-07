# Copy to Formula/collage.rb in the tap repo; fill url and sha256 per release.
class Collage < Formula
  desc "Arrange photos into a gap-free collage of any shape"
  homepage "https://github.com/vinodhalaharvi/collage"
  url "https://github.com/vinodhalaharvi/collage/archive/refs/tags/vVERSION.tar.gz"
  sha256 "SHA256"
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w -X main.version=#{version}"), "./cmd/collage"
  end

  test do
    assert_match "equal", shell_output("#{bin}/collage laws")
    assert_match version.to_s, shell_output("#{bin}/collage version")
  end
end
