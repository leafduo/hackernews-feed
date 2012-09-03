require 'builder'
require 'ruby-hackernews'
require 'readability'
require 'open-uri'

atom = Builder::XmlMarkup.new(:target => File.open('hn-feed.atom', 'w'), :indent => 2)
atom.instruct!
atom.feed "xmlns" => "http://www.w3.org/2005/Atom" do
    atom.title "Hacker News", :type => "text"
    atom.link :rel => "self", :href => "http://news.ycombinator.com/"
    atom.updated Time.now.utc.iso8601(0)
    atom.authors 'Customized by leafduo.com'
    atom.id "leafduo.com"
    RubyHackernews::Entry.all.each do |post|
        next if post.voting.score < 50 rescue "No score"
        atom.entry do
            title = post.link.title + ' (' + post.voting.score.to_s + ')'
            atom.title title
            puts title
            atom.link :href => post.link.href
            atom.id post.link.href
            #e.updated = post.time # FIXME: Due to a bug in ruby-hackernews, this can crash.
            begin
                original_content = open(post.link.href).read
                content = Readability::Document.new(original_content,
                                                    :tags => %w[div p img a], # These tags will be reserved
                                                    :attributes => %w[src href], # These attributes will be reserved
                                                    :remove_empty_nodes => false).content
                atom.content content, :type => "html"
            rescue
                atom.content = ''
            end
        end
    end
end
