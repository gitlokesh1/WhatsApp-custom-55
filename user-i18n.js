(() => {
  const languages = {
    en: 'English', hi: 'हिन्दी', pt: 'Português', es: 'Español', id: 'Bahasa Indonesia',
    bn: 'বাংলা', ur: 'اردو', ar: 'العربية'
  };
  const localeOrder = ['hi', 'pt', 'es', 'id', 'bn', 'ur', 'ar'];
  const phrases = {
    'Home': ['होम','Início','Inicio','Beranda','হোম','ہوم','الرئيسية'],
    'Tasks': ['कार्य','Tarefas','Tareas','Tugas','কাজ','کام','المهام'],
    'Referrals': ['रेफ़रल','Indicações','Referidos','Referensi','রেফারেল','ریفرلز','الإحالات'],
    'Profile': ['प्रोफ़ाइल','Perfil','Perfil','Profil','প্রোফাইল','پروفائل','الملف الشخصي'],
    'Support': ['सहायता','Suporte','Soporte','Dukungan','সহায়তা','مدد','الدعم'],
    'Sign out': ['साइन आउट','Sair','Cerrar sesión','Keluar','সাইন আউট','سائن آؤٹ','تسجيل الخروج'],
    'Secure WhatsApp task rewards': ['सुरक्षित WhatsApp कार्य पुरस्कार','Recompensas seguras por tarefas do WhatsApp','Recompensas seguras por tareas de WhatsApp','Imbalan tugas WhatsApp yang aman','নিরাপদ WhatsApp কাজের পুরস্কার','محفوظ WhatsApp ٹاسک انعامات','مكافآت مهام WhatsApp الآمنة'],
    'Hello, {name}': ['नमस्ते, {name}','Olá, {name}','Hola, {name}','Halo, {name}','হ্যালো, {name}','سلام، {name}','مرحبًا، {name}'],
    'Language': ['भाषा','Idioma','Idioma','Bahasa','ভাষা','زبان','اللغة'],
    'Grow together': ['साथ बढ़ें','Crescer juntos','Crecer juntos','Tumbuh bersama','একসাথে এগিয়ে চলুন','مل کر بڑھیں','لننمو معًا'],
    'Referral rewards': ['रेफ़रल पुरस्कार','Recompensas por indicação','Recompensas por referidos','Imbalan referensi','রেফারেল পুরস্কার','ریفرل انعامات','مكافآت الإحالة'],
    'Invite people to 88Task and track commissions from your network.': ['लोगों को 88Task पर आमंत्रित करें और अपने नेटवर्क का कमीशन देखें।','Convide pessoas para o 88Task e acompanhe as comissões da sua rede.','Invita personas a 88Task y consulta las comisiones de tu red.','Undang orang ke 88Task dan pantau komisi jaringan Anda.','88Task-এ মানুষকে আমন্ত্রণ জানান এবং নেটওয়ার্ক কমিশন দেখুন।','لوگوں کو 88Task پر مدعو کریں اور اپنے نیٹ ورک کا کمیشن دیکھیں۔','ادعُ الأشخاص إلى 88Task وتابع عمولات شبكتك.'],
    'Share referral link': ['रेफ़रल लिंक साझा करें','Compartilhar link de indicação','Compartir enlace de referido','Bagikan tautan referensi','রেফারেল লিংক শেয়ার করুন','ریفرل لنک شیئر کریں','مشاركة رابط الإحالة'],
    'Total referral commission': ['कुल रेफ़रल कमीशन','Comissão total de indicações','Comisión total por referidos','Total komisi referensi','মোট রেফারেল কমিশন','کل ریفرل کمیشن','إجمالي عمولة الإحالة'],
    'Commissions are credited in your country currency. Rates are configured by the administrator.': ['कमीशन आपकी देश की मुद्रा में जमा होता है। दरें एडमिन तय करता है।','As comissões são creditadas na moeda do seu país. As taxas são configuradas pelo administrador.','Las comisiones se acreditan en la moneda de tu país. El administrador configura las tasas.','Komisi dikreditkan dalam mata uang negara Anda. Tarif diatur administrator.','কমিশন আপনার দেশের মুদ্রায় জমা হয়। হার অ্যাডমিন নির্ধারণ করেন।','کمیشن آپ کے ملک کی کرنسی میں جمع ہوتا ہے۔ شرح ایڈمن مقرر کرتا ہے۔','تُضاف العمولات بعملة بلدك ويحدد المسؤول النسب.'],
    'Your invite code': ['आपका आमंत्रण कोड','Seu código de convite','Tu código de invitación','Kode undangan Anda','আপনার আমন্ত্রণ কোড','آپ کا دعوتی کوڈ','رمز دعوتك'],
    'Friends in your country can enter it while creating an account.': ['आपके देश के मित्र खाता बनाते समय इसे दर्ज कर सकते हैं।','Amigos do seu país podem usá-lo ao criar uma conta.','Amigos de tu país pueden usarlo al crear una cuenta.','Teman di negara Anda dapat memasukkannya saat membuat akun.','আপনার দেশের বন্ধুরা অ্যাকাউন্ট তৈরির সময় এটি ব্যবহার করতে পারেন।','آپ کے ملک کے دوست اکاؤنٹ بناتے وقت اسے درج کر سکتے ہیں۔','يمكن لأصدقائك في بلدك إدخاله عند إنشاء حساب.'],
    'Copy code': ['कोड कॉपी करें','Copiar código','Copiar código','Salin kode','কোড কপি করুন','کوڈ کاپی کریں','نسخ الرمز'],
    'Your network': ['आपका नेटवर्क','Sua rede','Tu red','Jaringan Anda','আপনার নেটওয়ার্ক','آپ کا نیٹ ورک','شبكتك'],
    'Direct referrals and the commission they have generated.': ['सीधे रेफ़रल और उनसे मिला कमीशन।','Indicações diretas e a comissão que elas geraram.','Referidos directos y la comisión que generaron.','Referensi langsung dan komisi yang dihasilkan.','সরাসরি রেফারেল এবং তাদের তৈরি কমিশন।','براہ راست ریفرلز اور ان سے حاصل کمیشن۔','الإحالات المباشرة والعمولات الناتجة عنها.'],
    '{count} referrals': ['{count} रेफ़रल','{count} indicações','{count} referidos','{count} referensi','{count} রেফারেল','{count} ریفرلز','{count} إحالات'],
    '{count} referral': ['{count} रेफ़रल','{count} indicação','{count} referido','{count} referensi','{count} রেফারেল','{count} ریفرل','إحالة واحدة'],
    'Referral code copied': ['रेफ़रल कोड कॉपी हुआ','Código de indicação copiado','Código de referido copiado','Kode referensi disalin','রেফারেল কোড কপি হয়েছে','ریفرل کوڈ کاپی ہو گیا','تم نسخ رمز الإحالة'],
    'Referral link copied': ['रेफ़रल लिंक कॉपी हुआ','Link de indicação copiado','Enlace de referido copiado','Tautan referensi disalin','রেফারেল লিংক কপি হয়েছে','ریفرل لنک کاپی ہو گیا','تم نسخ رابط الإحالة'],
    'Your earning journey': ['आपकी कमाई की यात्रा','Sua jornada de ganhos','Tu camino de ganancias','Perjalanan penghasilan Anda','আপনার আয়ের যাত্রা','آپ کی کمائی کا سفر','رحلة أرباحك'],
    'Build a rewarding daily habit.': ['रोज़ कमाने की अच्छी आदत बनाएं।','Crie um hábito diário recompensador.','Crea un hábito diario gratificante.','Bangun kebiasaan harian yang bermanfaat.','পুরস্কারময় দৈনিক অভ্যাস গড়ুন।','فائدہ مند روزانہ عادت بنائیں۔','ابنِ عادة يومية مجزية.'],
    'Create your account': ['अपना खाता बनाएं','Crie sua conta','Crea tu cuenta','Buat akun Anda','আপনার অ্যাকাউন্ট তৈরি করুন','اپنا اکاؤنٹ بنائیں','أنشئ حسابك'],
    'Start earning locally': ['स्थानीय रूप से कमाना शुरू करें','Comece a ganhar localmente','Empieza a ganar localmente','Mulai menghasilkan secara lokal','স্থানীয়ভাবে আয় শুরু করুন','مقامی طور پر کمانا شروع کریں','ابدأ الربح محليًا'],
    'Full name': ['पूरा नाम','Nome completo','Nombre completo','Nama lengkap','পুরো নাম','پورا نام','الاسم الكامل'],
    'Enter your full name': ['अपना पूरा नाम दर्ज करें','Digite seu nome completo','Ingresa tu nombre completo','Masukkan nama lengkap','আপনার পুরো নাম লিখুন','اپنا پورا نام درج کریں','أدخل اسمك الكامل'],
    'Earning country': ['कमाई का देश','País de ganhos','País de ganancias','Negara penghasilan','আয়ের দেশ','کمائی کا ملک','بلد الأرباح'],
    'Choose your country': ['अपना देश चुनें','Escolha seu país','Elige tu país','Pilih negara Anda','আপনার দেশ বেছে নিন','اپنا ملک منتخب کریں','اختر بلدك'],
    'Password': ['पासवर्ड','Senha','Contraseña','Kata sandi','পাসওয়ার্ড','پاس ورڈ','كلمة المرور'],
    'Referral code': ['रेफ़रल कोड','Código de indicação','Código de referido','Kode referensi','রেফারেল কোড','ریفرل کوڈ','رمز الإحالة'],
    '(optional)': ['(वैकल्पिक)','(opcional)','(opcional)','(opsional)','(ঐচ্ছিক)','(اختیاری)','(اختياري)'],
    'Create account': ['खाता बनाएं','Criar conta','Crear cuenta','Buat akun','অ্যাকাউন্ট তৈরি করুন','اکاؤنٹ بنائیں','إنشاء حساب'],
    'I agree to the': ['मैं सहमत हूं','Concordo com a','Acepto la','Saya menyetujui','আমি সম্মত','میں متفق ہوں','أوافق على'],
    'and': ['और','e','y','dan','এবং','اور','و'],
    'I agree to the Privacy Policy and Terms & Conditions.': ['मैं गोपनीयता नीति और नियम व शर्तों से सहमत हूं।','Concordo com a Política de Privacidade e os Termos e Condições.','Acepto la Política de Privacidad y los Términos y Condiciones.','Saya menyetujui Kebijakan Privasi serta Syarat dan Ketentuan.','আমি গোপনীয়তা নীতি ও শর্তাবলীতে সম্মত।','میں رازداری کی پالیسی اور شرائط و ضوابط سے متفق ہوں۔','أوافق على سياسة الخصوصية والشروط والأحكام.'],
    'Privacy Policy': ['गोपनीयता नीति','Política de Privacidade','Política de Privacidad','Kebijakan Privasi','গোপনীয়তা নীতি','رازداری کی پالیسی','سياسة الخصوصية'],
    'Terms & Conditions': ['नियम व शर्तें','Termos e Condições','Términos y Condiciones','Syarat dan Ketentuan','শর্তাবলী','شرائط و ضوابط','الشروط والأحكام'],
    'Already registered?': ['पहले से पंजीकृत हैं?','Já possui cadastro?','¿Ya estás registrado?','Sudah terdaftar?','ইতিমধ্যে নিবন্ধিত?','پہلے سے رجسٹرڈ ہیں؟','مسجل بالفعل؟'],
    'Sign in': ['साइन इन','Entrar','Iniciar sesión','Masuk','সাইন ইন','سائن ان','تسجيل الدخول'],
    'Your account': ['आपका खाता','Sua conta','Tu cuenta','Akun Anda','আপনার অ্যাকাউন্ট','آپ کا اکاؤنٹ','حسابك'],
    'Your profile': ['आपकी प्रोफ़ाइल','Seu perfil','Tu perfil','Profil Anda','আপনার প্রোফাইল','آپ کی پروفائل','ملفك الشخصي'],
    'Account details': ['खाता विवरण','Detalhes da conta','Detalles de la cuenta','Detail akun','অ্যাকাউন্টের বিবরণ','اکاؤنٹ کی تفصیلات','تفاصيل الحساب'],
    'Available balance': ['उपलब्ध बैलेंस','Saldo disponível','Saldo disponible','Saldo tersedia','উপলব্ধ ব্যালেন্স','دستیاب بیلنس','الرصيد المتاح'],
    'Wallet history': ['वॉलेट इतिहास','Histórico da carteira','Historial de la billetera','Riwayat dompet','ওয়ালেট ইতিহাস','والیٹ ہسٹری','سجل المحفظة'],
    'Reset password': ['पासवर्ड रीसेट करें','Redefinir senha','Restablecer contraseña','Atur ulang kata sandi','পাসওয়ার্ড রিসেট করুন','پاس ورڈ ری سیٹ کریں','إعادة تعيين كلمة المرور'],
    'Withdraw funds': ['राशि निकालें','Sacar fundos','Retirar fondos','Tarik dana','তহবিল উত্তোলন','رقم نکالیں','سحب الأموال'],
    'Search': ['खोजें','Pesquisar','Buscar','Cari','অনুসন্ধান','تلاش','بحث'],
    'Status': ['स्थिति','Status','Estado','Status','অবস্থা','حالت','الحالة'],
    'Refresh': ['रीफ़्रेश','Atualizar','Actualizar','Segarkan','রিফ্রেশ','ریفریش','تحديث'],
    'Load more': ['और लोड करें','Carregar mais','Cargar más','Muat lebih banyak','আরও লোড করুন','مزید لوڈ کریں','تحميل المزيد'],
    'Submit ticket': ['टिकट भेजें','Enviar chamado','Enviar ticket','Kirim tiket','টিকিট জমা দিন','ٹکٹ جمع کریں','إرسال تذكرة'],
    'Earned today': ['आज की कमाई','Ganho hoje','Ganado hoy','Diperoleh hari ini','আজকের আয়','آج کی کمائی','أرباح اليوم'],
    'Per task': ['प्रति कार्य','Por tarefa','Por tarea','Per tugas','প্রতি কাজ','فی کام','لكل مهمة'],
    'WhatsApp linked': ['WhatsApp जुड़े','WhatsApp conectados','WhatsApp vinculados','WhatsApp tertaut','WhatsApp যুক্ত','WhatsApp منسلک','WhatsApp مرتبط'],
    'View tasks': ['कार्य देखें','Ver tarefas','Ver tareas','Lihat tugas','কাজ দেখুন','کام دیکھیں','عرض المهام'],
    'Withdraw': ['निकालें','Sacar','Retirar','Tarik','উত্তোলন','رقم نکالیں','سحب'],
    'Daily mission': ['दैनिक मिशन','Missão diária','Misión diaria','Misi harian','দৈনিক মিশন','روزانہ مشن','المهمة اليومية'],
    'Finish today’s goal': ['आज का लक्ष्य पूरा करें','Conclua a meta de hoje','Completa la meta de hoy','Selesaikan target hari ini','আজকের লক্ষ্য পূরণ করুন','آج کا ہدف مکمل کریں','أكمل هدف اليوم'],
    'Lifetime journey': ['कुल यात्रा','Jornada total','Trayectoria total','Perjalanan keseluruhan','সামগ্রিক যাত্রা','مکمل سفر','المسيرة الكاملة'],
    'Lifetime tasks': ['कुल कार्य','Tarefas totais','Tareas totales','Total tugas','মোট কাজ','کل کام','إجمالي المهام'],
    'Copy invite': ['आमंत्रण कॉपी करें','Copiar convite','Copiar invitación','Salin undangan','আমন্ত্রণ কপি করুন','دعوت کاپی کریں','نسخ الدعوة'],
    'Referral network': ['रेफ़रल नेटवर्क','Rede de indicações','Red de referidos','Jaringan referensi','রেফারেল নেটওয়ার্ক','ریفرل نیٹ ورک','شبكة الإحالات'],
    'Country pending': ['देश लंबित','País pendente','País pendiente','Negara tertunda','দেশ অপেক্ষমাণ','ملک زیر التوا','البلد قيد الانتظار'],
    'Daily goal complete. Great work today!': ['दैनिक लक्ष्य पूरा हुआ। आज बहुत अच्छा काम!','Meta diária concluída. Ótimo trabalho hoje!','Meta diaria completada. ¡Excelente trabajo hoy!','Target harian selesai. Kerja bagus hari ini!','দৈনিক লক্ষ্য পূর্ণ হয়েছে। আজ দারুণ কাজ!','روزانہ ہدف مکمل۔ آج بہت اچھا کام!','اكتمل الهدف اليومي. عمل رائع اليوم!'],
    '{count} successful tasks left today.': ['आज {count} सफल कार्य बाकी हैं।','Restam {count} tarefas concluídas com sucesso hoje.','Quedan {count} tareas completadas correctamente hoy.','Tersisa {count} tugas berhasil hari ini.','আজ আরও {count}টি সফল কাজ বাকি।','آج {count} کامیاب کام باقی ہیں۔','تبقت {count} مهام ناجحة اليوم.'],
    'Highest level achieved': ['उच्चतम स्तर प्राप्त हुआ','Nível máximo alcançado','Nivel máximo alcanzado','Level tertinggi tercapai','সর্বোচ্চ স্তর অর্জিত','اعلیٰ ترین سطح حاصل ہو گئی','تم بلوغ أعلى مستوى'],
    '{count} tasks unlock the next level': ['{count} कार्य अगला स्तर खोलेंगे','{count} tarefas desbloqueiam o próximo nível','{count} tareas desbloquean el siguiente nivel','{count} tugas membuka level berikutnya','{count}টি কাজ পরবর্তী স্তর খুলবে','{count} کام اگلی سطح کھولیں گے','تفتح {count} مهام المستوى التالي'],
    'You reached the highest 88Task level.': ['आप 88Task के उच्चतम स्तर पर पहुंच गए हैं।','Você alcançou o nível máximo do 88Task.','Has alcanzado el nivel máximo de 88Task.','Anda telah mencapai level tertinggi 88Task.','আপনি 88Task-এর সর্বোচ্চ স্তরে পৌঁছেছেন।','آپ 88Task کی اعلیٰ ترین سطح پر پہنچ گئے ہیں۔','لقد وصلت إلى أعلى مستوى في 88Task.'],
    '{count} more successful tasks to advance.': ['आगे बढ़ने के लिए {count} और सफल कार्य।','Mais {count} tarefas concluídas com sucesso para avançar.','{count} tareas completadas correctamente más para avanzar.','{count} tugas berhasil lagi untuk maju.','এগিয়ে যেতে আরও {count}টি সফল কাজ।','آگے بڑھنے کے لیے مزید {count} کامیاب کام۔','{count} مهام ناجحة أخرى للتقدم.'],
    'Earning is currently paused for {country}.': ['{country} में कमाई अभी रुकी हुई है।','Os ganhos estão pausados no momento para {country}.','Las ganancias están pausadas actualmente para {country}.','Penghasilan saat ini dijeda untuk {country}.','{country}-এর জন্য আয় বর্তমানে স্থগিত।','{country} کے لیے کمائی فی الحال رکی ہوئی ہے۔','الأرباح متوقفة حاليًا في {country}.'],
    'No wallet activity': ['कोई वॉलेट गतिविधि नहीं','Nenhuma atividade na carteira','No hay actividad en la billetera','Tidak ada aktivitas dompet','কোনও ওয়ালেট কার্যকলাপ নেই','والیٹ میں کوئی سرگرمی نہیں','لا يوجد نشاط في المحفظة'],
    'Your transactions will appear here.': ['आपके लेनदेन यहां दिखाई देंगे।','Suas transações aparecerão aqui.','Tus transacciones aparecerán aquí.','Transaksi Anda akan muncul di sini.','আপনার লেনদেন এখানে দেখা যাবে।','آپ کے لین دین یہاں نظر آئیں گے۔','ستظهر معاملاتك هنا.'],
    'Wallet unavailable': ['वॉलेट उपलब्ध नहीं है','Carteira indisponível','Billetera no disponible','Dompet tidak tersedia','ওয়ালেট উপলভ্য নয়','والیٹ دستیاب نہیں','المحفظة غير متاحة'],
    'Other dashboard features remain available.': ['डैशबोर्ड की अन्य सुविधाएं उपलब्ध हैं।','Os outros recursos do painel continuam disponíveis.','Las demás funciones del panel siguen disponibles.','Fitur dasbor lainnya tetap tersedia.','ড্যাশবোর্ডের অন্যান্য সুবিধা উপলভ্য আছে।','ڈیش بورڈ کی دیگر خصوصیات دستیاب ہیں۔','تظل ميزات لوحة المعلومات الأخرى متاحة.'],
    'Wallet activity': ['वॉलेट गतिविधि','Atividade da carteira','Actividad de la billetera','Aktivitas dompet','ওয়ালেট কার্যক্রম','والیٹ سرگرمی','نشاط المحفظة'],
    'Recent transactions': ['हाल के लेनदेन','Transações recentes','Transacciones recientes','Transaksi terbaru','সাম্প্রতিক লেনদেন','حالیہ لین دین','المعاملات الأخيرة'],
    'View all': ['सभी देखें','Ver tudo','Ver todo','Lihat semua','সব দেখুন','سب دیکھیں','عرض الكل'],
    'Verified task rewards': ['सत्यापित कार्य पुरस्कार','Recompensas por tarefas verificadas','Recompensas por tareas verificadas','Imbalan tugas terverifikasi','যাচাইকৃত কাজের পুরস্কার','تصدیق شدہ کام کے انعامات','مكافآت المهام الموثقة'],
    'Welcome back': ['वापसी पर स्वागत','Boas-vindas','Bienvenido de nuevo','Selamat datang kembali','আবার স্বাগতম','خوش آمدید','مرحبًا بعودتك'],
    'Sign in to keep earning': ['कमाई जारी रखने के लिए साइन इन करें','Entre para continuar ganhando','Inicia sesión para seguir ganando','Masuk untuk terus menghasilkan','আয় চালিয়ে যেতে সাইন ইন করুন','کمائی جاری رکھنے کے لیے سائن ان کریں','سجل الدخول لمواصلة الربح'],
    'User ID': ['उपयोगकर्ता आईडी','ID do usuário','ID de usuario','ID pengguna','ব্যবহারকারী আইডি','صارف آئی ڈی','معرّف المستخدم'],
    'Create an account': ['खाता बनाएं','Criar uma conta','Crear una cuenta','Buat akun','অ্যাকাউন্ট তৈরি করুন','اکاؤنٹ بنائیں','إنشاء حساب'],
    'Account security': ['खाता सुरक्षा','Segurança da conta','Seguridad de la cuenta','Keamanan akun','অ্যাকাউন্ট নিরাপত্তা','اکاؤنٹ سیکیورٹی','أمان الحساب'],
    'Back to profile': ['प्रोफ़ाइल पर वापस','Voltar ao perfil','Volver al perfil','Kembali ke profil','প্রোফাইলে ফিরুন','پروفائل پر واپس','العودة إلى الملف الشخصي'],
    'Set a new password': ['नया पासवर्ड सेट करें','Definir nova senha','Definir nueva contraseña','Atur kata sandi baru','নতুন পাসওয়ার্ড সেট করুন','نیا پاس ورڈ مقرر کریں','تعيين كلمة مرور جديدة'],
    'Current password': ['वर्तमान पासवर्ड','Senha atual','Contraseña actual','Kata sandi saat ini','বর্তমান পাসওয়ার্ড','موجودہ پاس ورڈ','كلمة المرور الحالية'],
    'New password': ['नया पासवर्ड','Nova senha','Nueva contraseña','Kata sandi baru','নতুন পাসওয়ার্ড','نیا پاس ورڈ','كلمة المرور الجديدة'],
    'Confirm new password': ['नए पासवर्ड की पुष्टि','Confirmar nova senha','Confirmar nueva contraseña','Konfirmasi kata sandi baru','নতুন পাসওয়ার্ড নিশ্চিত করুন','نئے پاس ورڈ کی تصدیق','تأكيد كلمة المرور الجديدة'],
    'Update password': ['पासवर्ड अपडेट करें','Atualizar senha','Actualizar contraseña','Perbarui kata sandi','পাসওয়ার্ড আপডেট করুন','پاس ورڈ اپ ڈیٹ کریں','تحديث كلمة المرور'],
    'Security tips': ['सुरक्षा सुझाव','Dicas de segurança','Consejos de seguridad','Tips keamanan','নিরাপত্তা পরামর্শ','سیکیورٹی تجاویز','نصائح الأمان'],
    'Name': ['नाम','Nome','Nombre','Nama','নাম','نام','الاسم'],
    'Currency': ['मुद्रा','Moeda','Moneda','Mata uang','মুদ্রা','کرنسی','العملة'],
    'Current level': ['वर्तमान स्तर','Nível atual','Nivel actual','Level saat ini','বর্তমান স্তর','موجودہ سطح','المستوى الحالي'],
    'Current message rate': ['वर्तमान संदेश दर','Taxa atual por mensagem','Tarifa actual por mensaje','Tarif pesan saat ini','বর্তমান বার্তা হার','موجودہ پیغام شرح','سعر الرسالة الحالي'],
    'We are here to help': ['हम सहायता के लिए हैं','Estamos aqui para ajudar','Estamos aquí para ayudarte','Kami siap membantu','আমরা সাহায্য করতে আছি','ہم مدد کے لیے حاضر ہیں','نحن هنا للمساعدة'],
    'Customer support': ['ग्राहक सहायता','Atendimento ao cliente','Atención al cliente','Dukungan pelanggan','গ্রাহক সহায়তা','کسٹمر سپورٹ','دعم العملاء'],
    'Subject': ['विषय','Assunto','Asunto','Subjek','বিষয়','موضوع','الموضوع'],
    'Details': ['विवरण','Detalhes','Detalles','Detail','বিবরণ','تفصیلات','التفاصيل'],
    'Quick help': ['त्वरित सहायता','Ajuda rápida','Ayuda rápida','Bantuan cepat','দ্রুত সহায়তা','فوری مدد','مساعدة سريعة'],
    'My tickets': ['मेरे टिकट','Meus chamados','Mis tickets','Tiket saya','আমার টিকিট','میرے ٹکٹ','تذاكري'],
    'Verified earning tasks': ['सत्यापित कमाई कार्य','Tarefas de ganhos verificadas','Tareas de ganancias verificadas','Tugas penghasilan terverifikasi','যাচাইকৃত আয়ের কাজ','تصدیق شدہ کمائی کے کام','مهام الأرباح الموثقة'],
    'Available tasks': ['उपलब्ध कार्य','Tarefas disponíveis','Tareas disponibles','Tugas tersedia','উপলব্ধ কাজ','دستیاب کام','المهام المتاحة'],
    'Choose a sending account': ['भेजने वाला खाता चुनें','Escolha uma conta de envio','Elige una cuenta de envío','Pilih akun pengirim','প্রেরণকারী অ্যাকাউন্ট বেছে নিন','بھیجنے والا اکاؤنٹ منتخب کریں','اختر حساب الإرسال'],
    'Manage accounts': ['खाते प्रबंधित करें','Gerenciar contas','Gestionar cuentas','Kelola akun','অ্যাকাউন্ট পরিচালনা','اکاؤنٹس سنبھالیں','إدارة الحسابات'],
    'Your money': ['आपकी राशि','Seu dinheiro','Tu dinero','Uang Anda','আপনার অর্থ','آپ کی رقم','أموالك'],
    'Wallet': ['वॉलेट','Carteira','Billetera','Dompet','ওয়ালেট','والیٹ','المحفظة'],
    'Transactions shown': ['दिखाए गए लेनदेन','Transações exibidas','Transacciones mostradas','Transaksi ditampilkan','প্রদর্শিত লেনদেন','دکھائے گئے لین دین','المعاملات المعروضة'],
    'Direction': ['दिशा','Direção','Dirección','Arah','দিক','سمت','الاتجاه'],
    'All': ['सभी','Todos','Todos','Semua','সব','تمام','الكل'],
    'Credits': ['क्रेडिट','Créditos','Créditos','Kredit','ক্রেডিট','کریڈٹس','الإضافات'],
    'Debits': ['डेबिट','Débitos','Débitos','Debit','ডেবিট','ڈیبٹس','الخصومات'],
    'Type': ['प्रकार','Tipo','Tipo','Jenis','ধরন','قسم','النوع'],
    'All types': ['सभी प्रकार','Todos os tipos','Todos los tipos','Semua jenis','সব ধরন','تمام اقسام','كل الأنواع'],
    'Task reward': ['कार्य पुरस्कार','Recompensa de tarefa','Recompensa de tarea','Imbalan tugas','কাজের পুরস্কার','کام کا انعام','مكافأة المهمة'],
    'Referral commission': ['रेफ़रल कमीशन','Comissão de indicação','Comisión de referido','Komisi referensi','রেফারেল কমিশন','ریفرل کمیشن','عمولة الإحالة'],
    'Bonus': ['बोनस','Bônus','Bono','Bonus','বোনাস','بونس','مكافأة'],
    'Bonus reversal': ['बोनस वापसी','Estorno de bônus','Reversión de bono','Pembalikan bonus','বোনাস ফেরত','بونس واپسی','عكس المكافأة'],
    'Admin debit': ['एडमिन डेबिट','Débito administrativo','Débito administrativo','Debit admin','অ্যাডমিন ডেবিট','ایڈمن ڈیبٹ','خصم إداري'],
    'Withdrawal': ['निकासी','Saque','Retiro','Penarikan','উত্তোলন','رقم نکالنا','سحب'],
    'Withdrawal refund': ['निकासी वापसी','Reembolso de saque','Reembolso de retiro','Pengembalian penarikan','উত্তোলন ফেরত','رقم نکالنے کی واپسی','استرداد السحب'],
    'All statuses': ['सभी स्थितियां','Todos os status','Todos los estados','Semua status','সব অবস্থা','تمام حالتیں','كل الحالات'],
    'Completed': ['पूर्ण','Concluído','Completado','Selesai','সম্পন্ন','مکمل','مكتمل'],
    'Pending': ['लंबित','Pendente','Pendiente','Tertunda','অপেক্ষমাণ','زیر التوا','قيد الانتظار'],
    'Refunded': ['वापस किया','Reembolsado','Reembolsado','Dikembalikan','ফেরত','واپس شدہ','مسترد'],
    'Secure connection': ['सुरक्षित कनेक्शन','Conexão segura','Conexión segura','Koneksi aman','নিরাপদ সংযোগ','محفوظ کنکشن','اتصال آمن'],
    'WhatsApp accounts': ['WhatsApp खाते','Contas do WhatsApp','Cuentas de WhatsApp','Akun WhatsApp','WhatsApp অ্যাকাউন্ট','WhatsApp اکاؤنٹس','حسابات WhatsApp'],
    'Maximum 3 accounts': ['अधिकतम 3 खाते','Máximo de 3 contas','Máximo 3 cuentas','Maksimum 3 akun','সর্বোচ্চ ৩ অ্যাকাউন্ট','زیادہ سے زیادہ 3 اکاؤنٹس','3 حسابات كحد أقصى'],
    'Link a new account': ['नया खाता जोड़ें','Vincular nova conta','Vincular una cuenta nueva','Tautkan akun baru','নতুন অ্যাকাউন্ট যুক্ত করুন','نیا اکاؤنٹ منسلک کریں','ربط حساب جديد'],
    'WhatsApp phone number': ['WhatsApp फ़ोन नंबर','Número do WhatsApp','Número de WhatsApp','Nomor WhatsApp','WhatsApp ফোন নম্বর','WhatsApp فون نمبر','رقم هاتف WhatsApp'],
    'Generate pairing code': ['पेयरिंग कोड बनाएं','Gerar código de pareamento','Generar código de vinculación','Buat kode penautan','পেয়ারিং কোড তৈরি করুন','پیئرنگ کوڈ بنائیں','إنشاء رمز الربط'],
    'How to connect': ['कैसे जोड़ें','Como conectar','Cómo conectar','Cara menghubungkan','কীভাবে সংযোগ করবেন','کیسے منسلک کریں','كيفية الربط'],
    'Open WhatsApp': ['WhatsApp खोलें','Abra o WhatsApp','Abre WhatsApp','Buka WhatsApp','WhatsApp খুলুন','WhatsApp کھولیں','افتح WhatsApp'],
    'Open Linked devices': ['लिंक किए गए डिवाइस खोलें','Abra Aparelhos conectados','Abre Dispositivos vinculados','Buka Perangkat tertaut','লিংক করা ডিভাইস খুলুন','منسلک ڈیوائسز کھولیں','افتح الأجهزة المرتبطة'],
    'Enter the code': ['कोड दर्ज करें','Digite o código','Ingresa el código','Masukkan kode','কোড লিখুন','کوڈ درج کریں','أدخل الرمز'],
    'Your accounts': ['आपके खाते','Suas contas','Tus cuentas','Akun Anda','আপনার অ্যাকাউন্ট','آپ کے اکاؤنٹس','حساباتك'],
    'Secure payouts': ['सुरक्षित भुगतान','Pagamentos seguros','Pagos seguros','Pembayaran aman','নিরাপদ পেমেন্ট','محفوظ ادائیگیاں','مدفوعات آمنة'],
    'New request': ['नया अनुरोध','Nova solicitação','Nueva solicitud','Permintaan baru','নতুন অনুরোধ','نئی درخواست','طلب جديد'],
    'Payout channel': ['भुगतान चैनल','Canal de pagamento','Canal de pago','Kanal pembayaran','পেমেন্ট চ্যানেল','ادائیگی چینل','قناة الدفع'],
    'Amount': ['राशि','Valor','Importe','Jumlah','পরিমাণ','رقم','المبلغ'],
    'Request withdrawal': ['निकासी अनुरोध करें','Solicitar saque','Solicitar retiro','Ajukan penarikan','উত্তোলনের অনুরোধ করুন','رقم نکالنے کی درخواست','طلب سحب'],
    'Withdrawal history': ['निकासी इतिहास','Histórico de saques','Historial de retiros','Riwayat penarikan','উত্তোলন ইতিহাস','رقم نکالنے کی ہسٹری','سجل السحوبات'],
    'Back': ['वापस','Voltar','Volver','Kembali','ফিরে যান','واپس','رجوع'],
    'Back to registration': ['पंजीकरण पर वापस','Voltar ao cadastro','Volver al registro','Kembali ke pendaftaran','নিবন্ধনে ফিরুন','رجسٹریشن پر واپس','العودة إلى التسجيل'],
    'Legal': ['कानूनी','Jurídico','Legal','Hukum','আইনি','قانونی','قانوني'],
    'Last updated: 9 September 2026': ['अंतिम अपडेट: 9 सितंबर 2026','Última atualização: 9 de setembro de 2026','Última actualización: 9 de septiembre de 2026','Terakhir diperbarui: 9 September 2026','সর্বশেষ আপডেট: ৯ সেপ্টেম্বর ২০২৬','آخری اپ ڈیٹ: 9 ستمبر 2026','آخر تحديث: 9 سبتمبر 2026'],
    'Information we collect': ['हम कौन-सी जानकारी लेते हैं','Informações que coletamos','Información que recopilamos','Informasi yang kami kumpulkan','আমরা যে তথ্য সংগ্রহ করি','ہم کون سی معلومات جمع کرتے ہیں','المعلومات التي نجمعها'],
    'How we use information': ['हम जानकारी का उपयोग कैसे करते हैं','Como usamos as informações','Cómo usamos la información','Cara kami menggunakan informasi','আমরা তথ্য যেভাবে ব্যবহার করি','ہم معلومات کیسے استعمال کرتے ہیں','كيف نستخدم المعلومات'],
    'Sharing and service providers': ['साझाकरण और सेवा प्रदाता','Compartilhamento e prestadores de serviço','Uso compartido y proveedores de servicio','Berbagi dan penyedia layanan','শেয়ারিং ও সেবা প্রদানকারী','شیئرنگ اور سروس فراہم کنندگان','المشاركة ومقدمو الخدمات'],
    'Retention and security': ['संग्रह अवधि और सुरक्षा','Retenção e segurança','Retención y seguridad','Penyimpanan dan keamanan','সংরক্ষণ ও নিরাপত্তা','مدتِ تحفظ اور سیکیورٹی','الاحتفاظ والأمان'],
    'Your choices and rights': ['आपके विकल्प और अधिकार','Suas escolhas e direitos','Tus opciones y derechos','Pilihan dan hak Anda','আপনার পছন্দ ও অধিকার','آپ کے اختیارات اور حقوق','خياراتك وحقوقك'],
    'Contact and changes': ['संपर्क और बदलाव','Contato e alterações','Contacto y cambios','Kontak dan perubahan','যোগাযোগ ও পরিবর্তন','رابطہ اور تبدیلیاں','التواصل والتغييرات'],
    'Eligibility and accounts': ['पात्रता और खाते','Elegibilidade e contas','Elegibilidad y cuentas','Kelayakan dan akun','যোগ্যতা ও অ্যাকাউন্ট','اہلیت اور اکاؤنٹس','الأهلية والحسابات'],
    'Tasks and acceptable use': ['कार्य और स्वीकार्य उपयोग','Tarefas e uso aceitável','Tareas y uso aceptable','Tugas dan penggunaan yang diperbolehkan','কাজ ও গ্রহণযোগ্য ব্যবহার','کام اور قابل قبول استعمال','المهام والاستخدام المقبول'],
    'Rewards, referrals, and withdrawals': ['पुरस्कार, रेफ़रल और निकासी','Recompensas, indicações e saques','Recompensas, referidos y retiros','Imbalan, referensi, dan penarikan','পুরস্কার, রেফারেল ও উত্তোলন','انعامات، ریفرلز اور رقم نکالنا','المكافآت والإحالات والسحب'],
    'Suspension and termination': ['निलंबन और समाप्ति','Suspensão e encerramento','Suspensión y terminación','Penangguhan dan penghentian','স্থগিতকরণ ও সমাপ্তি','معطلی اور خاتمہ','التعليق والإنهاء'],
    'Disclaimers and liability': ['अस्वीकरण और दायित्व','Isenções e responsabilidade','Descargos y responsabilidad','Penafian dan tanggung jawab','দায়বর্জন ও দায়','اعلانِ لاتعلقی اور ذمہ داری','إخلاء المسؤولية والمسؤولية القانونية'],
    'Changes and contact': ['बदलाव और संपर्क','Alterações e contato','Cambios y contacto','Perubahan dan kontak','পরিবর্তন ও যোগাযোগ','تبدیلیاں اور رابطہ','التغييرات والتواصل']
  };

  const dictionaries = { en: {} };
  for (const code of localeOrder) dictionaries[code] = {};
  for (const [source, values] of Object.entries(phrases)) {
    localeOrder.forEach((code, index) => { dictionaries[code][source] = values[index]; });
  }

  const stored = localStorage.getItem('88task_user_lang');
  const browserLanguage = (navigator.language || 'en').split('-')[0].toLowerCase();
  let language = languages[stored] ? stored : (languages[browserLanguage] ? browserLanguage : 'en');
  const templates = Object.keys(phrases).filter(value => value.includes('{')).map(source => {
    const names = [...source.matchAll(/\{(\w+)\}/g)].map(match => match[1]);
    const expression = '^' + source.split(/\{\w+\}/).map(part => part.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('(.+?)') + '$';
    return { source, names, regex: new RegExp(expression) };
  });

  function translate(source, values) {
    let template = dictionaries[language]?.[source] || source;
    if (!values) {
      const matched = templates.find(item => item.regex.test(source));
      if (matched) {
        const captures = source.match(matched.regex);
        values = Object.fromEntries(matched.names.map((name, index) => [name, captures[index + 1]]));
        template = dictionaries[language]?.[matched.source] || matched.source;
      }
    }
    return template.replace(/\{(\w+)\}/g, (_, key) => values?.[key] ?? `{${key}}`);
  }

  function translateText(node) {
    if (!node.nodeValue || ['SCRIPT', 'STYLE'].includes(node.parentElement?.tagName)) return;
    const trimmed = node.nodeValue.trim();
    if (!trimmed) return;
    const translated = translate(trimmed);
    if (translated !== trimmed) node.nodeValue = node.nodeValue.replace(trimmed, translated);
  }

  function translateTree(root) {
    if (root.nodeType === Node.TEXT_NODE) return translateText(root);
    const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
    while (walker.nextNode()) translateText(walker.currentNode);
    root.querySelectorAll?.('[placeholder],[aria-label],[title]').forEach(element => {
      for (const attribute of ['placeholder', 'aria-label', 'title']) {
        const value = element.getAttribute(attribute);
        if (value) element.setAttribute(attribute, translate(value));
      }
    });
  }

  function initializeSelectors(root = document) {
    root.querySelectorAll?.('[data-user-language]').forEach(selector => {
      if (selector.dataset.languageReady) return;
      selector.dataset.languageReady = 'true';
      selector.innerHTML = Object.entries(languages).map(([code, name]) => `<option value="${code}">${name}</option>`).join('');
      selector.value = language;
      selector.setAttribute('aria-label', translate('Language'));
      selector.addEventListener('change', () => {
        localStorage.setItem('88task_user_lang', selector.value);
        location.reload();
      });
    });
  }

  function init() {
    document.documentElement.lang = language;
    document.documentElement.dir = ['ar', 'ur'].includes(language) ? 'rtl' : 'ltr';
    if (!document.querySelector('main[data-user-page]') && !document.querySelector('[data-user-language]')) {
      const selector = document.createElement('select');
      selector.className = 'u-language u-language-floating';
      selector.dataset.userLanguage = '';
      document.body.prepend(selector);
    }
    queueMicrotask(() => {
      initializeSelectors();
      translateTree(document.body);
    });
  }

  window.uT = translate;
  window.uLanguage = () => language;
  window.uTranslate = translateTree;
  window.uInitLanguageSelectors = initializeSelectors;
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init);
  else init();
})();
