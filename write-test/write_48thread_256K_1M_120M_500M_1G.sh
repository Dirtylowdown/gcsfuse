FILE_AMOUNT=10 BLOCK_SIZE=16K FILE_SIZE=256K MODE=write NUMJOBS=48 fio write.fio
FILE_AMOUNT=10 BLOCK_SIZE=16K FILE_SIZE=256K MODE=randwrite NUMJOBS=48 fio write.fio
FILE_AMOUNT=10 BLOCK_SIZE=1M FILE_SIZE=1M MODE=write NUMJOBS=48 fio write.fio
FILE_AMOUNT=10 BLOCK_SIZE=1M FILE_SIZE=1M MODE=randwrite NUMJOBS=48 fio write.fio
FILE_AMOUNT=10 BLOCK_SIZE=1M FILE_SIZE=120M MODE=write NUMJOBS=48 fio write.fio
FILE_AMOUNT=10 BLOCK_SIZE=1M FILE_SIZE=120M MODE=randwrite NUMJOBS=48 fio write.fio
FILE_AMOUNT=10 BLOCK_SIZE=1M FILE_SIZE=500M MODE=write NUMJOBS=48 fio write.fio
FILE_AMOUNT=10 BLOCK_SIZE=1M FILE_SIZE=500M MODE=randwrite NUMJOBS=48 fio write.fio
FILE_AMOUNT=10 BLOCK_SIZE=1M FILE_SIZE=1G MODE=write NUMJOBS=48 fio write.fio
FILE_AMOUNT=10 BLOCK_SIZE=1M FILE_SIZE=1G MODE=randwrite NUMJOBS=48 fio write.fio