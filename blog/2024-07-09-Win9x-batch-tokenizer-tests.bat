: Test cases for the no longer documented craziness that is the batch line
: tokenizer in Windows 9x's `COMMAND.COM`. Cases marked with ❌ fail with a
: "Bad command or filename" error.

: ✅ argc = 64 ("echo" + 63 words)
echo 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3>NUL
: ✅ Same with trailing space
echo 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 >NUL
: ✅ Same with duplicated spaces
echo 1  2  3  4  5  6  7  8  9  0  1  2  3  4  5  6  7  8  9  0  1  2  3  4  5  6  7  8  9  0  1  2  3  4  5  6  7  8  9  0  1  2  3  4  5  6  7  8  9  0  1  2  3  4  5  6  7  8  9  0  1  2  3  >NUL
: ❌ argc = 65 ("echo" + 64 words)
echo 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4>NUL
: ✅ argc 64 ("echo" + 63 slashes)
echo ///////////////////////////////////////////////////////////////>NUL
: ❌ argc = 65 ("echo" + 64 slashes) (Works on DOS!)
echo ////////////////////////////////////////////////////////////////>NUL
: ✅ argc = 64 ("echo" + 63 "switches")
echo /o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o>NUL
: ❌ argc = 65 ("echo" + 64 "switches")
echo /o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/o/>NUL
: ✅ argc = 64 ("echo" + 63 space-separated switches)
echo /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o>NUL
: ❌ argc = 65 ("echo" + 64 space-separated switches)
echo /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o /o />NUL
: ✅ argc = 64 ("echo" + 31 assignments + 1 word)
echo a=0 b=1 c=2 d=3 e=4 f=5 g=6 h=7 i=8 j=9 k=0 l=1 m=2 n=3 o=4 p=5 q=6 r=7 s=8 t=9 u=0 v=1 w=2 x=3 y=4 z=5 a=0 b=1 c=2 d=3 e=4 f>NUL
: ❌ argc = 65 ("echo" + 32 assignments)
echo a=0 b=1 c=2 d=3 e=4 f=5 g=6 h=7 i=8 j=9 k=0 l=1 m=2 n=3 o=4 p=5 q=6 r=7 s=8 t=9 u=0 v=1 w=2 x=3 y=4 z=5 a=0 b=1 c=2 d=3 e=4 f=5>NUL
: ✅ argc = 64 ("echo" + 63 unpaired assignments)
echo =1 =2 =3 =4 =5 =6 =7 =8 =9 =0 =1 =2 =3 =4 =5 =6 =7 =8 =9 =0 =1 =2 =3 =4 =5 =6 =7 =8 =9 =0 =1 =2 =3 =4 =5 =6 =7 =8 =9 =0 =1 =2 =3 =4 =5 =6 =7 =8 =9 =0 =1 =2 =3 =4 =5 =6 =7 =8 =9 =0 =1 =2 =3>NUL
: ❌ argc = 65 ("echo" + 64 unpaired assignments)
echo =1 =2 =3 =4 =5 =6 =7 =8 =9 =0 =1 =2 =3 =4 =5 =6 =7 =8 =9 =0 =1 =2 =3 =4 =5 =6 =7 =8 =9 =0 =1 =2 =3 =4 =5 =6 =7 =8 =9 =0 =1 =2 =3 =4 =5 =6 =7 =8 =9 =0 =1 =2 =3 =4 =5 =6 =7 =8 =9 =0 =1 =2 =3 =4>NUL
