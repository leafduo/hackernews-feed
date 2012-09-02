require 'atom'
require 'feedzirra'
require 'ruby-hackernews'
require 'readability'
require 'open-uri'

feed = Atom::Feed.new do |f|
    f.title = "Hacker News"
    f.links << Atom::Link.new(:href => "http://news.ycombinator.com/")
    f.updated = Time.now
    f.authors << Atom::Person.new(:name => 'Customized by leafduo.com')
    f.id = "leafduo.com"
    RubyHackernews::Entry.all.each do |post|
        f.entries << Atom::Entry.new do |e|
            e.title = post.link.title + ' (' + post.voting.score.to_s + ')'
            puts e.title
            e.links << Atom::Link.new(:href => post.link.href)
            e.id = post.link.href
            #e.updated = post.time # FIXME: Due to a bug in ruby-hackernews, this can crash.
            begin
                original_content = open(post.link.href).read
                e.content = Readability::Document.new(original_content).content
            rescue
                e.content = ''
            end
            #e.summary = "Some text."
        end
    end
end

File.open('hn-feed.atom', 'w') {|f| f.write(feed.to_xml) }
