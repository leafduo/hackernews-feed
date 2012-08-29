require 'atom'
require 'feedzirra'
require 'ruby-hackernews'

feed = Atom::Feed.new do |f|
    f.title = "Hacker News"
    f.links << Atom::Link.new(:href => "http://news.ycombinator.com/")
    f.updated = Time.now
    f.authors << Atom::Person.new(:name => 'Customized by leafduo.com')
    f.id = "leafduo.com"
    RubyHackernews::Entry.all.each do |post|
        f.entries << Atom::Entry.new do |e|
            e.title = post.link.title
            e.links << Atom::Link.new(:href => post.link.href)
            e.id = post.link.href
            #e.updated = post.time # FIXME: Due to a bug in ruby-hackernews, this can crash.
            #e.summary = "Some text."
        end
    end
end

File.open('hn-feed.atom', 'w') {|f| f.write(feed.to_xml) }
