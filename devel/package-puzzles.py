#!/usr/bin/env python3

import argparse
import binascii
import glob
import hashlib
import io
import json
import logging
import moth
import os
import shutil
import string
import sys
import tempfile
import zipfile

def write_kv_pairs(ziphandle, filename, kv):
    """ Write out a sorted map to file
    :param ziphandle: a zipfile object
    :param filename: The filename to write within the zipfile object
    :param kv:  the map to write out
    :return:
    """
    filehandle = io.StringIO()
    for key in sorted(kv.keys()):
        if type(kv[key])  == type([]):
            for val in kv[key]:
                filehandle.write("%s %s\n" % (key, val))
        else:
            filehandle.write("%s %s\n" % (key, kv[key]))
    filehandle.seek(0)
    ziphandle.writestr(filename, filehandle.read())
    
def escape(s):
    return s.replace('&', '&amp;').replace('<', '&lt;').replace('>', '&gt;')
    
def generate_html(ziphandle, puzzle, puzzledir, category, points, authors, files):
    html_content = io.StringIO()
    file_content = io.StringIO()
    if files:
        file_content.write(
'''        <section id="files">
            <h2>Associated files:</h2>
            <ul>
''')
        for fn in files:
            file_content.write('                <li><a href="{fn}">{efn}</a></li>\n'.format(fn=fn, efn=escape(fn)))
        file_content.write(
'''            </ul>
        </section>
''')
    scripts = ['<script src="{}"></script>'.format(s) for s in puzzle.scripts]
        
    html_content.write(
'''<!DOCTYPE html>
<html>
    <head>
        <meta charset="UTF-8">
        <meta name="viewport" content="width=device-width">
        <title>{category} {points}</title>
        <link rel="stylesheet" href="../../style.css">
        {scripts}
    </head>
    <body>
        <h1>{category} for {points} points</h1>
        <section id="readme">
{body}        </section>
{file_content}        <section id="form">
            <form id="puzzler" action="../../cgi-bin/puzzler.cgi" method="get" accept-charset="utf-8" autocomplete="off">
                <input type="hidden" name="c" value="{category}">
                <input type="hidden" name="p" value="{points}">
                <div>Team hash:<input name="t" size="8"></div>
                <div>Answer:<input name="a" id="answer" size="20"></div>
                <input type="submit" value="submit">
            </form>
        </section>
        <address>Puzzle by <span class="authors" data-handle="{authors}">{authors}</span></address>
    </body>
</html>'''.format(
            category=category,
            points=points,
            body=puzzle.html_body(),
            file_content=file_content.getvalue(),
            authors=', '.join(authors),
            scripts='\n'.join(scripts),
	)
    )
    ziphandle.writestr(os.path.join(puzzledir, 'index.html'), html_content.getvalue())

def build_category(categorydir, outdir):
    zipfileraw = tempfile.NamedTemporaryFile(delete=False)
    zf = zipfile.ZipFile(zipfileraw, 'x')

    category_seed = binascii.b2a_hex(os.urandom(20))
    puzzles_dict = {}
    secrets = {}

    categoryname = os.path.basename(categorydir.strip(os.sep))
    seedfn = os.path.join("category_seed.txt")
    zipfilename = os.path.join(outdir, "%s.mb" % categoryname)
    logging.info("Building {} from {}".format(zipfilename, categorydir))

    if os.path.exists(zipfilename):
        # open and gather some state
        existing = zipfile.ZipFile(zipfilename, 'r')
        try:
            category_seed = existing.open(seedfn).read().strip()
        except:
            pass
        existing.close()
    logging.debug("Using PRNG seed {}".format(category_seed))

    zf.writestr(seedfn, category_seed)

    cat = moth.Category(categorydir, category_seed)
    mapping = {}
    answers = {}
    answerdyn = {}
    summary = {}
    unlockers = {}
    for puzzle in cat:
        logging.info("Processing point value {}".format(puzzle.points))

        hashmap = hashlib.sha1(category_seed)
        hashmap.update(str(puzzle.points).encode('utf-8'))
        puzzlehash = hashmap.hexdigest()
        
        mapping[puzzle.points] = puzzlehash
        answers[puzzle.points] = puzzle.answers
        answerdyn[puzzle.points] = puzzle.answerdyn
        summary[puzzle.points] = puzzle.summary
        unlockers[puzzle.points] = puzzle.unlockers

        puzzledir = os.path.join('content', puzzlehash)
        files = []
        for fn, f in puzzle.files.items():
            if f.visible:
                files.append(fn)
            payload = f.stream.read()
            zf.writestr(os.path.join(puzzledir, fn), payload)

        puzzleanswerdyndir = os.path.join('answerdyn', puzzlehash)
        answerdyngrader = []
        for fn, f in puzzle.answerdyngrader.items():
            if f.visible:
                answerdyngrader.append(fn)
            payload = f.stream.read()
            zf.writestr(os.path.join(puzzleanswerdyndir, fn), payload)


        puzzledict = {
            'authors': puzzle.authors,
            'hashes': puzzle.hashes(),
            'files': files,
            'body': puzzle.html_body(),
        }
        puzzlejson = json.dumps(puzzledict)
        zf.writestr(os.path.join(puzzledir, 'puzzle.json'), puzzlejson)
        generate_html(zf, puzzle, puzzledir, categoryname, puzzle.points, puzzle.get_authors(), files)

    write_kv_pairs(zf, 'map.txt', mapping)
    write_kv_pairs(zf, 'answers.txt', answers)
    write_kv_pairs(zf, 'answerdyn.txt', answerdyn)
    write_kv_pairs(zf, 'summaries.txt', summary)
    write_kv_pairs(zf, 'unlockers.txt', unlockers)

    # clean up
    zf.close()

    shutil.move(zipfileraw.name, zipfilename)
    
   
if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='Build a category package')
    parser.add_argument('outdir', help='Output directory')
    parser.add_argument('categorydirs', nargs='+', help='Directory of category source')
    args = parser.parse_args()

    logging.basicConfig(level=logging.DEBUG)

    for categorydir in args.categorydirs:
        build_category(categorydir, args.outdir)

